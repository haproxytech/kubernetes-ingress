// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"fmt"
	"hash/fnv"
	"net"
	"path"
	"strconv"
	"strings"

	"github.com/haproxytech/client-native/v2/misc"
	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

const (
	defaultCaptureLen      = 128
	defaultSSLRedirectCode = 302
)

var sslRedirectEnabled map[string]struct{}
var rateLimitTables map[string]rateLimitTable

func (c *HAProxyController) handleBlacklisting(ingress *store.Ingress) error {
	//  Get and validate annotations
	annBlacklist, _ := c.Store.GetValueFromAnnotations("blacklist", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annBlacklist == nil {
		return nil
	}
	value := strings.Replace(annBlacklist.Value, ",", " ", -1)
	mapFiles := c.cfg.MapFiles
	listKey := hashStrToUint(annBlacklist.Value)
	for _, address := range strings.Fields(value) {
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				return fmt.Errorf("incorrect value for blacklist annotation in ingress '%s'", ingress.Name)
			}
		}
		mapFiles.AppendRow(listKey, address)
	}

	if len(ingress.Rules) == 0 {
		logger.Debugf("Ingress %s/%s: Skipping blacklist configuration, no rules defined", ingress.Namespace, ingress.Name)
		return nil
	}

	// Update rules
	status := setStatus(ingress.Status, annBlacklist.Status)

	listMapFile := path.Join(HAProxyMapDir, strconv.FormatUint(listKey, 10)) + ".lst"
	key := hashStrToUint(fmt.Sprintf("%s-%s", BLACKLIST, annBlacklist.Value))
	if status != EMPTY {
		c.cfg.FrontendRulesModified[HTTP] = true
		c.cfg.FrontendRulesModified[TCP] = true
		if status == DELETED {
			logger.Debugf("Ingress %s/%s: Deleting blacklist configuration", ingress.Namespace, ingress.Name)
			return nil
		}
		logger.Debugf("Ingress %s/%s: Configuring blacklist annotation", ingress.Namespace, ingress.Name)
	}
	for hostname, rule := range ingress.Rules {
		if rule.Status != DELETED {
			for path := range rule.Paths {
				mapFiles.AppendRow(key, hostname+path)
			}
		}
	}

	mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "deny",
		DenyStatus: 403,
		Cond:       "if",
		CondTest:   makeACL(fmt.Sprintf(" { src -f %s }", listMapFile), mapFile),
	}
	tcpRule := models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Type:     "content",
		Action:   "reject",
		Cond:     "if",
		CondTest: fmt.Sprintf("{ req_ssl_sni -f %s } { src -f %s }", mapFile, listMapFile),
	}
	c.cfg.FrontendHTTPReqRules[BLACKLIST][key] = httpRule
	c.cfg.FrontendTCPRules[BLACKLIST][key] = tcpRule

	return nil
}

func (c *HAProxyController) handleHTTPRedirect(ingress *store.Ingress) error {
	//  Get and validate annotations
	var err error
	toEnable := false
	annSSLRedirect, _ := c.Store.GetValueFromAnnotations("ssl-redirect", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	annRedirectCode, _ := c.Store.GetValueFromAnnotations("ssl-redirect-code", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	_, enabled := sslRedirectEnabled[ingress.Namespace+ingress.Name]
	if annSSLRedirect == nil {
		if len(ingress.TLS) > 0 {
			toEnable = true
		} else if !enabled {
			// Nothing to do
			return nil
		}
	} else {
		switch annSSLRedirect.Status {
		case DELETED:
			if len(ingress.TLS) > 0 {
				toEnable = true
			}
		default:
			if toEnable, err = utils.GetBoolValue(annSSLRedirect.Value, "ssl-redirect"); err != nil {
				return err
			}
		}
	}
	var sslRedirectCode int64
	if sslRedirectCode, err = strconv.ParseInt(annRedirectCode.Value, 10, 64); err != nil {
		sslRedirectCode = defaultSSLRedirectCode
	}

	if len(ingress.Rules) == 0 {
		logger.Debugf("Ingress %s/%s: Skipping redirect configuration, no rules defined", ingress.Namespace, ingress.Name)
		return nil
	}

	// Update Rules
	key := hashStrToUint(fmt.Sprintf("%s-%d", SSL_REDIRECT, sslRedirectCode))
	mapFiles := c.cfg.MapFiles
	// Disable Redirect
	if !toEnable {
		if enabled {
			delete(sslRedirectEnabled, ingress.Namespace+ingress.Name)
			c.cfg.FrontendRulesModified[HTTP] = true
		}
		return nil
	}
	//Enable Redirect
	for hostname, rule := range ingress.Rules {
		if rule.Status != DELETED {
			for path := range rule.Paths {
				mapFiles.AppendRow(key, hostname+path)
			}
		}
	}
	mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "redirect",
		RedirCode:  sslRedirectCode,
		RedirValue: "https",
		RedirType:  "scheme",
		Cond:       "if",
		CondTest:   makeACL(" !{ ssl_fc }", mapFile),
	}
	c.cfg.FrontendHTTPReqRules[SSL_REDIRECT][key] = httpRule

	if !enabled {
		c.cfg.FrontendRulesModified[HTTP] = true
		sslRedirectEnabled[ingress.Namespace+ingress.Name] = struct{}{}
	}
	return nil
}

func (c *HAProxyController) handleRateLimiting(ingress *store.Ingress) error {
	//  Get and validate annotations
	annRateLimitReq, _ := c.Store.GetValueFromAnnotations("rate-limit-requests", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annRateLimitReq == nil {
		return nil
	}
	reqsLimit, err := strconv.ParseInt(annRateLimitReq.Value, 10, 64)
	if err != nil {
		return err
	}
	// Following annotations have default values
	annRateLimitPeriod, _ := c.Store.GetValueFromAnnotations("rate-limit-period", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	rateLimitPeriod, err := utils.ParseTime(annRateLimitPeriod.Value)
	if err != nil {
		return err
	}
	annRateLimitSize, _ := c.Store.GetValueFromAnnotations("rate-limit-size", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	rateLimitSize := misc.ParseSize(annRateLimitSize.Value)

	annRateLimitCode, _ := c.Store.GetValueFromAnnotations("rate-limit-status-code", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	rateLimitCode, err := utils.ParseInt(annRateLimitCode.Value)
	if err != nil {
		return err
	}

	if len(ingress.Rules) == 0 {
		logger.Debugf("Ingress %s/%s: Skipping rate-limit configuration, no rules defined", ingress.Namespace, ingress.Name)
		return nil
	}

	// Update rules
	status := setStatus(ingress.Status, annRateLimitReq.Status)
	if status == EMPTY {
		if annRateLimitPeriod.Status != EMPTY || annRateLimitCode.Status != EMPTY {
			status = MODIFIED
		}
	}
	mapFiles := c.cfg.MapFiles
	reqsKey := hashStrToUint(fmt.Sprintf("%s-%d-%d", RATE_LIMIT, *rateLimitPeriod, reqsLimit))
	trackKey := hashStrToUint(fmt.Sprintf("%s-%d", RATE_LIMIT, *rateLimitPeriod))
	tableName := fmt.Sprintf("RateLimit-%d", *rateLimitPeriod)
	if status != EMPTY {
		c.cfg.FrontendRulesModified[HTTP] = true
		if status == DELETED {
			logger.Debugf("Ingress %s/%s: Deleting rate-limit-requests configuration", ingress.Namespace, ingress.Name)
			delete(rateLimitTables, tableName)
			return nil
		}
		logger.Debugf("Ingress %s/%s: Configuring rate-limit-requests annotation", ingress.Namespace, ingress.Name)
	}
	for hostname, rule := range ingress.Rules {
		if rule.Status != DELETED {
			for path := range rule.Paths {
				mapFiles.AppendRow(reqsKey, hostname+path)
				mapFiles.AppendRow(trackKey, hostname+path)
			}
		}
	}
	rateLimitTables[tableName] = rateLimitTable{
		size:   rateLimitSize,
		period: rateLimitPeriod,
	}
	trackMapFile := path.Join(HAProxyMapDir, strconv.FormatUint(trackKey, 10)) + ".lst"
	httpTrackRule := models.HTTPRequestRule{
		Index:         utils.PtrInt64(0),
		Type:          "track-sc0",
		TrackSc0Key:   "src",
		TrackSc0Table: tableName,
		Cond:          "if",
		CondTest:      makeACL("", trackMapFile),
	}
	reqsMapFile := path.Join(HAProxyMapDir, strconv.FormatUint(reqsKey, 10)) + ".lst"
	httpDenyRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(1),
		Type:       "deny",
		DenyStatus: rateLimitCode,
		Cond:       "if",
		CondTest:   makeACL(fmt.Sprintf(" { sc0_http_req_rate(%s) gt %d }", tableName, reqsLimit), reqsMapFile),
	}
	c.cfg.FrontendHTTPReqRules[RATE_LIMIT][trackKey] = httpTrackRule
	c.cfg.FrontendHTTPReqRules[RATE_LIMIT][reqsKey] = httpDenyRule
	return nil
}

func (c *HAProxyController) handleRequestCapture(ingress *store.Ingress) error {
	//  Get and validate annotations
	annReqCapture, _ := c.Store.GetValueFromAnnotations("request-capture", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	annCaptureLen, _ := c.Store.GetValueFromAnnotations("request-capture-len", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annReqCapture == nil {
		return nil
	}
	var captureLen int64
	var err error
	if annCaptureLen != nil {
		captureLen, err = strconv.ParseInt(annCaptureLen.Value, 10, 64)
		if err != nil {
			captureLen = defaultCaptureLen
		}
		if annCaptureLen.Status == DELETED {
			captureLen = defaultCaptureLen
		}
	} else {
		captureLen = defaultCaptureLen
	}

	if len(ingress.Rules) == 0 {
		logger.Debugf("Ingress %s/%s: Skipping capture configuration, no rules defined", ingress.Namespace, ingress.Name)
		return nil
	}

	// Update rules
	status := setStatus(ingress.Status, annReqCapture.Status)
	mapFiles := c.cfg.MapFiles
	for _, sample := range strings.Split(annReqCapture.Value, "\n") {
		if sample == "" {
			continue
		}
		key := hashStrToUint(fmt.Sprintf("%s-%s-%d", REQUEST_CAPTURE, sample, captureLen))
		if status != EMPTY {
			c.cfg.FrontendRulesModified[HTTP] = true
			c.cfg.FrontendRulesModified[TCP] = true
			if status == DELETED {
				logger.Debugf("Ingress %s/%s: Deleting configuration for '%s' request capture ", ingress.Namespace, ingress.Name, sample)
				break
			}
			logger.Debugf("Ingress %s/%s: Configuring request capture for '%s'", ingress.Namespace, ingress.Name, sample)
		}
		for hostname, rule := range ingress.Rules {
			if rule.Status != DELETED {
				for path := range rule.Paths {
					mapFiles.AppendRow(key, hostname+path)
				}
			}
		}

		mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
		httpRule := models.HTTPRequestRule{
			Index:         utils.PtrInt64(0),
			Type:          "capture",
			CaptureSample: sample,
			Cond:          "if",
			CaptureLen:    captureLen,
			CondTest:      makeACL("", mapFile),
		}
		tcpRule := models.TCPRequestRule{
			Index:      utils.PtrInt64(0),
			Type:       "content",
			Action:     "capture",
			CaptureLen: captureLen,
			Expr:       sample,
			Cond:       "if",
			CondTest:   fmt.Sprintf("{ req_ssl_sni -f %s }", mapFile),
		}
		c.cfg.FrontendHTTPReqRules[REQUEST_CAPTURE][key] = httpRule
		c.cfg.FrontendTCPRules[REQUEST_CAPTURE][key] = tcpRule
	}

	return err
}

func (c *HAProxyController) handleRequestSetHdr(ingress *store.Ingress) error {
	//  Get and validate annotations
	annSetHdr, err := c.Store.GetValueFromAnnotations("request-set-header", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annSetHdr == nil {
		return nil
	}

	if len(ingress.Rules) == 0 {
		logger.Debugf("Ingress %s/%s: Skipping request-set-hdr configuration, no rules defined", ingress.Namespace, ingress.Name)
		return nil
	}

	// Update rules
	status := setStatus(ingress.Status, annSetHdr.Status)
	mapFiles := c.cfg.MapFiles
	for _, param := range strings.Split(annSetHdr.Value, "\n") {
		parts := strings.Fields(param)
		if len(parts) != 2 {
			logger.Errorf("incorrect value '%s' in request-set-header annotation", param)
			continue
		}
		key := hashStrToUint(fmt.Sprintf("%s-%s-%s", REQUEST_SET_HEADER, parts[0], parts[1]))
		if status != EMPTY {
			c.cfg.FrontendRulesModified[HTTP] = true
			if status == DELETED {
				logger.Debugf("Ingress %s/%s: Deleting configuration for request set '%s' header ", ingress.Namespace, ingress.Name, param)
				break
			}
			logger.Debugf("Ingress %s/%s: Configuring request set '%s' header ", ingress.Namespace, ingress.Name, param)
		}
		for hostname, rule := range ingress.Rules {
			if rule.Status != DELETED {
				for path := range rule.Paths {
					mapFiles.AppendRow(key, hostname+path)
				}
			}
		}

		mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
		httpRule := models.HTTPRequestRule{
			Index:     utils.PtrInt64(0),
			Type:      "set-header",
			HdrName:   parts[0],
			HdrFormat: parts[1],
			Cond:      "if",
			CondTest:  makeACL("", mapFile),
		}
		c.cfg.FrontendHTTPReqRules[REQUEST_SET_HEADER][key] = httpRule
	}

	return err
}

func (c *HAProxyController) handleResponseSetHdr(ingress *store.Ingress) error {
	//  Get and validate annotations
	annSetHdr, err := c.Store.GetValueFromAnnotations("response-set-header", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annSetHdr == nil {
		return nil
	}

	// Update rules
	if len(ingress.Rules) == 0 {
		logger.Debugf("Ingress %s/%s: Skipping response-set-header configuration, no rules defined", ingress.Namespace, ingress.Name)
		return nil
	}
	status := setStatus(ingress.Status, annSetHdr.Status)
	mapFiles := c.cfg.MapFiles
	for _, param := range strings.Split(annSetHdr.Value, "\n") {
		parts := strings.Fields(param)
		if len(parts) != 2 {
			logger.Errorf("incorrect value '%s' in response-set-header annotation", param)
			continue
		}
		key := hashStrToUint(fmt.Sprintf("%s-%s-%s", RESPONSE_SET_HEADER, parts[0], parts[1]))
		if status != EMPTY {
			c.cfg.FrontendRulesModified[HTTP] = true
			if status == DELETED {
				break
			}
		}
		for hostname, rule := range ingress.Rules {
			if rule.Status != DELETED {
				for path := range rule.Paths {
					mapFiles.AppendRow(key, hostname+path)
				}
			}
		}

		mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
		httpRule := models.HTTPResponseRule{
			Index:     utils.PtrInt64(0),
			Type:      "set-header",
			HdrName:   parts[0],
			HdrFormat: parts[1],
			Cond:      "if",
			CondTest:  makeACL("", mapFile),
		}
		c.cfg.FrontendHTTPRspRules[RESPONSE_SET_HEADER][key] = httpRule
	}

	return err
}

func (c *HAProxyController) handleWhitelisting(ingress *store.Ingress) error {
	//  Get and validate annotations
	annWhitelist, _ := c.Store.GetValueFromAnnotations("whitelist", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annWhitelist == nil {
		return nil
	}
	mapFiles := c.cfg.MapFiles
	listKey := hashStrToUint(annWhitelist.Value)
	value := strings.Replace(annWhitelist.Value, ",", " ", -1)
	for _, address := range strings.Fields(value) {
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				return fmt.Errorf("incorrect value for whitelist annotation in ingress '%s'", ingress.Name)
			}
		}
		mapFiles.AppendRow(listKey, address)
	}

	if len(ingress.Rules) == 0 {
		logger.Debugf("Ingress %s/%s: Skipping whitelist configuration, no rules defined", ingress.Namespace, ingress.Name)
		return nil
	}

	// Update rules
	status := setStatus(ingress.Status, annWhitelist.Status)
	listMapFile := path.Join(HAProxyMapDir, strconv.FormatUint(listKey, 10)) + ".lst"
	key := hashStrToUint(fmt.Sprintf("%s-%s", WHITELIST, annWhitelist.Value))
	if status != EMPTY {
		c.cfg.FrontendRulesModified[HTTP] = true
		c.cfg.FrontendRulesModified[TCP] = true
		if status == DELETED {
			logger.Debugf("Ingress %s/%s: Deleting whitelist configuration", ingress.Namespace, ingress.Name)
			return nil
		}
		logger.Debugf("Ingress %s/%s: Configuring whitelist configuration", ingress.Namespace, ingress.Name)
	}
	for hostname, rule := range ingress.Rules {
		if rule.Status != DELETED {
			for path := range rule.Paths {
				mapFiles.AppendRow(key, hostname+path)
			}
		}
	}

	mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "deny",
		DenyStatus: 403,
		Cond:       "if",
		CondTest:   makeACL(fmt.Sprintf(" !{ src -f %s }", listMapFile), mapFile),
	}
	tcpRule := models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Type:     "content",
		Action:   "reject",
		Cond:     "if",
		CondTest: fmt.Sprintf("{ req_ssl_sni -f %s } !{ src -f %s }", mapFile, listMapFile),
	}
	c.cfg.FrontendHTTPReqRules[WHITELIST][key] = httpRule
	c.cfg.FrontendTCPRules[WHITELIST][key] = tcpRule

	return nil
}

func hashStrToUint(s string) uint64 {
	h := fnv.New64a()
	_, err := h.Write([]byte(strings.ToLower(s)))
	logger.Error(err)
	return h.Sum64()
}

// Return status for ingress annotations
func setStatus(ingressStatus, annStatus store.Status) store.Status {
	if ingressStatus == DELETED || annStatus == DELETED {
		return DELETED
	}
	if ingressStatus == EMPTY && annStatus == EMPTY {
		return EMPTY
	}
	return MODIFIED
}

func makeACL(acl string, mapFile string) (result string) {
	result = fmt.Sprintf("{ var(txn.host),concat(,txn.path) -m beg -f %s }", mapFile) + acl
	result += " or " + fmt.Sprintf("{ var(txn.host) -f %s }", mapFile) + acl
	result += " or " + fmt.Sprintf("{ var(txn.path) -m beg -f %s }", mapFile) + acl
	return result
}
