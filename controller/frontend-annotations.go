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

	"github.com/haproxytech/client-native/misc"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models"
)

const (
	defaultCaptureLen      = 128
	defaultSSLRedirectCode = 302
)

var sslRedirectEnabled map[string]struct{}
var rateLimitTables map[string]rateLimitTable

func (c *HAProxyController) handleBlacklisting(ingress *Ingress) error {
	//  Get and validate annotations
	annBlacklist, _ := GetValueFromAnnotations("blacklist", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	if annBlacklist == nil {
		return nil
	}
	value := strings.Replace(annBlacklist.Value, ",", " ", -1)
	for _, address := range strings.Fields(value) {
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				return fmt.Errorf("incorrect value for blacklist annotation in ingress '%s'", ingress.Name)
			}
		}
	}

	// Update rules
	status := setStatus(ingress.Status, annBlacklist.Status)
	mapFiles := c.cfg.MapFiles
	key := hashStrToUint(fmt.Sprintf("%s-%s", BLACKLIST, annBlacklist.Value))
	if status != EMPTY {
		mapFiles.Modified(key)
		c.cfg.FrontendRulesStatus[HTTP] = MODIFIED
		c.cfg.FrontendRulesStatus[TCP] = MODIFIED
		if status == DELETED {
			return nil
		}
	}
	for hostname := range ingress.Rules {
		mapFiles.AppendHost(key, hostname)
	}

	mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "deny",
		DenyStatus: 403,
		Cond:       "if",
		CondTest:   fmt.Sprintf("{ req.hdr(Host) -f %s } { src %s }", mapFile, value),
	}
	tcpRule := models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Type:     "content",
		Action:   "reject",
		Cond:     "if",
		CondTest: fmt.Sprintf("{ req_ssl_sni -f %s } { src %s }", mapFile, value),
	}
	c.cfg.FrontendHTTPReqRules[BLACKLIST][key] = httpRule
	c.cfg.FrontendTCPRules[BLACKLIST][key] = tcpRule

	return nil
}

func (c *HAProxyController) handleHTTPRedirect(ingress *Ingress) error {
	//  Get and validate annotations
	var err error
	toEnable := false
	annSSLRedirect, _ := GetValueFromAnnotations("ssl-redirect", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	annRedirectCode, _ := GetValueFromAnnotations("ssl-redirect-code", ingress.Annotations, c.cfg.ConfigMap.Annotations)
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

	// Update Rules
	key := hashStrToUint(fmt.Sprintf("%s-%d", SSL_REDIRECT, sslRedirectCode))
	mapFiles := c.cfg.MapFiles
	// Disable Redirect
	if !toEnable {
		if enabled {
			delete(sslRedirectEnabled, ingress.Namespace+ingress.Name)
			mapFiles.Modified(key)
			c.cfg.FrontendRulesStatus[HTTP] = MODIFIED
		}
		return nil
	}
	//Enable Redirect
	for hostname := range ingress.Rules {
		mapFiles.AppendHost(key, hostname)
	}
	mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "redirect",
		RedirCode:  sslRedirectCode,
		RedirValue: "https",
		RedirType:  "scheme",
		Cond:       "if",
		//TODO: provide option to do strict host matching
		CondTest: fmt.Sprintf("{ req.hdr(host),field(1,:) -f %s } !{ ssl_fc }", mapFile),
	}
	c.cfg.FrontendHTTPReqRules[SSL_REDIRECT][key] = httpRule

	if !enabled {
		mapFiles.Modified(key)
		c.cfg.FrontendRulesStatus[HTTP] = MODIFIED
		sslRedirectEnabled[ingress.Namespace+ingress.Name] = struct{}{}
	}
	return nil
}

func (c *HAProxyController) handleProxyProtocol() error {
	//  Get and validate annotations
	annProxyProtocol, _ := GetValueFromAnnotations("proxy-protocol", c.cfg.ConfigMap.Annotations)
	if annProxyProtocol == nil {
		return nil
	}
	value := strings.Replace(annProxyProtocol.Value, ",", " ", -1)
	for _, address := range strings.Fields(value) {
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				return fmt.Errorf("incorrect value for proxy-protocol annotation ")
			}
		}
	}

	// Get Rules status
	status := annProxyProtocol.Status

	// Update rules
	// Since this is a Configmap Annotation ONLY, no need to
	// track ingress hosts in Map file
	if status != EMPTY {
		c.cfg.FrontendRulesStatus[TCP] = MODIFIED
		if status == DELETED {
			return nil
		}
	}

	tcpRule := models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Type:     "connection",
		Action:   "expect-proxy layer4",
		Cond:     "if",
		CondTest: fmt.Sprintf("{ src %s }", value),
	}
	c.cfg.FrontendTCPRules[PROXY_PROTOCOL][0] = tcpRule

	return nil
}

func (c *HAProxyController) handleRateLimiting(ingress *Ingress) error {
	//  Get and validate annotations
	annRateLimitReq, _ := GetValueFromAnnotations("rate-limit-requests", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	if annRateLimitReq == nil {
		return nil
	}
	reqsLimit, err := strconv.ParseInt(annRateLimitReq.Value, 10, 64)
	if err != nil {
		return err
	}
	// Following annotaitons have default values
	annRateLimitPeriod, _ := GetValueFromAnnotations("rate-limit-period", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	rateLimitPeriod, err := utils.ParseTime(annRateLimitPeriod.Value)
	if err != nil {
		return err
	}
	annRateLimitSize, _ := GetValueFromAnnotations("rate-limit-size", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	rateLimitSize := misc.ParseSize(annRateLimitSize.Value)

	// Update rules
	var status Status
	if annRateLimitReq.Status != EMPTY {
		status = setStatus(ingress.Status, annRateLimitReq.Status)
	} else {
		status = setStatus(ingress.Status, annRateLimitPeriod.Status)
	}
	mapFiles := c.cfg.MapFiles
	reqsKey := hashStrToUint(fmt.Sprintf("%s-%d-%d", RATE_LIMIT, *rateLimitPeriod, reqsLimit))
	trackKey := hashStrToUint(fmt.Sprintf("%s-%d", RATE_LIMIT, *rateLimitPeriod))
	tableName := fmt.Sprintf("RateLimit-%d", *rateLimitPeriod)
	if status != EMPTY {
		mapFiles.Modified(reqsKey)
		mapFiles.Modified(trackKey)
		c.cfg.FrontendRulesStatus[HTTP] = MODIFIED
		if status == DELETED {
			delete(rateLimitTables, tableName)
			return nil
		}
	}
	for hostname := range ingress.Rules {
		mapFiles.AppendHost(reqsKey, hostname)
		mapFiles.AppendHost(trackKey, hostname)
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
		CondTest:      fmt.Sprintf("{ req.hdr(Host) -f %s }", trackMapFile),
	}
	reqsMapFile := path.Join(HAProxyMapDir, strconv.FormatUint(reqsKey, 10)) + ".lst"
	httpDenyRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(1),
		Type:       "deny",
		DenyStatus: 403,
		Cond:       "if",
		CondTest:   fmt.Sprintf("{ req.hdr(Host) -f %s } { sc0_http_req_rate(%s) gt %d }", reqsMapFile, tableName, reqsLimit),
	}
	c.cfg.FrontendHTTPReqRules[RATE_LIMIT][trackKey] = httpTrackRule
	c.cfg.FrontendHTTPReqRules[RATE_LIMIT][reqsKey] = httpDenyRule
	return nil
}

func (c *HAProxyController) handleRequestCapture(ingress *Ingress) error {
	//  Get and validate annotations
	annReqCapture, _ := GetValueFromAnnotations("request-capture", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	annCaptureLen, _ := GetValueFromAnnotations("request-capture-len", ingress.Annotations, c.cfg.ConfigMap.Annotations)
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

	// Update rules
	status := setStatus(ingress.Status, annReqCapture.Status)
	mapFiles := c.cfg.MapFiles
	for _, sample := range strings.Split(annReqCapture.Value, "\n") {
		if sample == "" {
			continue
		}
		key := hashStrToUint(fmt.Sprintf("%s-%s-%d", REQUEST_CAPTURE, sample, captureLen))
		if status != EMPTY {
			mapFiles.Modified(key)
			c.cfg.FrontendRulesStatus[HTTP] = MODIFIED
			c.cfg.FrontendRulesStatus[TCP] = MODIFIED
			if status == DELETED {
				break
			}
		}
		for hostname := range ingress.Rules {
			mapFiles.AppendHost(key, hostname)
		}

		mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
		httpRule := models.HTTPRequestRule{
			Index:         utils.PtrInt64(0),
			Type:          "capture",
			CaptureSample: sample,
			Cond:          "if",
			CaptureLen:    captureLen,
			CondTest:      fmt.Sprintf("{ req.hdr(Host) -f %s }", mapFile),
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

func (c *HAProxyController) handleRequestSetHdr(ingress *Ingress) error {
	//  Get and validate annotations
	annSetHdr, err := GetValueFromAnnotations("request-set-header", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	if annSetHdr == nil {
		return nil
	}

	// Update rules
	status := setStatus(ingress.Status, annSetHdr.Status)
	mapFiles := c.cfg.MapFiles
	for _, param := range strings.Split(annSetHdr.Value, "\n") {
		parts := strings.Fields(param)
		if len(parts) != 2 {
			utils.LogErr(fmt.Errorf("incorrect value '%s' in request-set-header annotation", param))
			continue
		}
		key := hashStrToUint(fmt.Sprintf("%s-%s-%s", REQUEST_SET_HEADER, parts[0], parts[1]))
		if status != EMPTY {
			mapFiles.Modified(key)
			c.cfg.FrontendRulesStatus[HTTP] = MODIFIED
			if status == DELETED {
				break
			}
		}
		for hostname := range ingress.Rules {
			mapFiles.AppendHost(key, hostname)
		}

		mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
		httpRule := models.HTTPRequestRule{
			Index:     utils.PtrInt64(0),
			Type:      "set-header",
			HdrName:   parts[0],
			HdrFormat: parts[1],
			Cond:      "if",
			CondTest:  fmt.Sprintf("{ req.hdr(Host) -f %s }", mapFile),
		}
		c.cfg.FrontendHTTPReqRules[REQUEST_SET_HEADER][key] = httpRule
	}

	return err
}

func (c *HAProxyController) handleResponseSetHdr(ingress *Ingress) error {
	//  Get and validate annotations
	annSetHdr, err := GetValueFromAnnotations("response-set-header", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	if annSetHdr == nil {
		return nil
	}

	// Update rules
	status := setStatus(ingress.Status, annSetHdr.Status)
	mapFiles := c.cfg.MapFiles
	for _, param := range strings.Split(annSetHdr.Value, "\n") {
		parts := strings.Fields(param)
		if len(parts) != 2 {
			utils.LogErr(fmt.Errorf("incorrect value '%s' in response-set-header annotation", param))
			continue
		}
		key := hashStrToUint(fmt.Sprintf("%s-%s-%s", RESPONSE_SET_HEADER, parts[0], parts[1]))
		if status != EMPTY {
			mapFiles.Modified(key)
			c.cfg.FrontendRulesStatus[HTTP] = MODIFIED
			if status == DELETED {
				break
			}
		}
		for hostname := range ingress.Rules {
			mapFiles.AppendHost(key, hostname)
		}

		mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
		httpRule := models.HTTPResponseRule{
			Index:     utils.PtrInt64(0),
			Type:      "set-header",
			HdrName:   parts[0],
			HdrFormat: parts[1],
			Cond:      "if",
			CondTest:  fmt.Sprintf("{ req.hdr(Host) -f %s }", mapFile),
		}
		c.cfg.FrontendHTTPRspRules[RESPONSE_SET_HEADER][key] = httpRule
	}

	return err
}

func (c *HAProxyController) handleWhitelisting(ingress *Ingress) error {
	//  Get and validate annotations
	annWhitelist, _ := GetValueFromAnnotations("whitelist", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	if annWhitelist == nil {
		return nil
	}
	value := strings.Replace(annWhitelist.Value, ",", " ", -1)
	for _, address := range strings.Fields(value) {
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				return fmt.Errorf("incorrect value for whitelist annotation in ingress '%s'", ingress.Name)
			}
		}
	}

	// Update rules
	status := setStatus(ingress.Status, annWhitelist.Status)
	mapFiles := c.cfg.MapFiles
	key := hashStrToUint(fmt.Sprintf("%s-%s", WHITELIST, annWhitelist.Value))
	if status != EMPTY {
		mapFiles.Modified(key)
		c.cfg.FrontendRulesStatus[HTTP] = MODIFIED
		c.cfg.FrontendRulesStatus[TCP] = MODIFIED
		if status == DELETED {
			return nil
		}
	}
	for hostname := range ingress.Rules {
		mapFiles.AppendHost(key, hostname)
	}

	mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "deny",
		DenyStatus: 403,
		Cond:       "if",
		CondTest:   fmt.Sprintf("{ req.hdr(Host) -f %s } !{ src %s }", mapFile, value),
	}
	tcpRule := models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Type:     "content",
		Action:   "reject",
		Cond:     "if",
		CondTest: fmt.Sprintf("{ req_ssl_sni -f %s } !{ src %s }", mapFile, value),
	}
	c.cfg.FrontendHTTPReqRules[WHITELIST][key] = httpRule
	c.cfg.FrontendTCPRules[WHITELIST][key] = tcpRule

	return nil
}

func hashStrToUint(s string) uint64 {
	h := fnv.New64a()
	_, err := h.Write([]byte(strings.ToLower(s)))
	utils.LogErr(err)
	return h.Sum64()
}

// Return status for ingress annotations
func setStatus(ingressStatus, annStatus Status) Status {
	if ingressStatus == DELETED || annStatus == DELETED {
		return DELETED
	}
	if ingressStatus == EMPTY && annStatus == EMPTY {
		return EMPTY
	}
	return MODIFIED
}
