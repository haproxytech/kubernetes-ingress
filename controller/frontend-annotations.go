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
	"net"
	"strconv"
	"strings"

	"github.com/haproxytech/client-native/v2/misc"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type rateLimitTable struct {
	size   *int64
	period *int64
}

var rateLimitTables map[string]rateLimitTable

func (c *HAProxyController) handleIngressAnnotations(ingress *store.Ingress) {
	logger.Error(c.handleRateLimiting(ingress))
	logger.Error(c.handleRequestCapture(ingress))
	logger.Error(c.handleRequestPathRewrite(ingress))
	logger.Error(c.handleRequestSetHost(ingress))
	logger.Error(c.handleRequestSetHdr(ingress))
	logger.Error(c.handleResponseSetHdr(ingress))
	logger.Error(c.handleBlacklisting(ingress))
	logger.Error(c.handleWhitelisting(ingress))
	logger.Error(c.handleHTTPRedirect(ingress))
}

func (c *HAProxyController) handleBlacklisting(ingress *store.Ingress) error {
	//  Get annotation status
	annBlacklist, _ := c.Store.GetValueFromAnnotations("blacklist", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annBlacklist == nil {
		return nil
	}
	if ingress.Status == DELETED || annBlacklist.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting blacklist configuration", ingress.Namespace, ingress.Name)
		return nil
	}
	// Validate annotation
	ips, _ := haproxy.NewMapID(annBlacklist.Value)
	for _, address := range strings.Split(annBlacklist.Value, ",") {
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				logger.Errorf("incorrect address '%s' in blacklist annotation in ingress '%s'", address, ingress.Name)
				continue
			}
		}
		c.cfg.MapFiles.AppendRow(ips, address)
	}
	// Configure annotation
	logger.Debugf("Ingress %s/%s: Configuring blacklist annotation", ingress.Namespace, ingress.Name)
	match, _ := haproxy.NewMapID(fmt.Sprintf("%d-%s", haproxy.REQ_DENY, annBlacklist.Value))
	for hostname, rule := range ingress.Rules {
		if rule.Status != DELETED {
			for path := range rule.Paths {
				c.cfg.MapFiles.AppendRow(match, hostname+path)
			}
		}
	}
	reqBlackList := rules.ReqDeny{
		Ingress: match,
		SrcIPs:  ips,
	}
	return c.cfg.HAProxyRules.AddRule(reqBlackList, match, FrontendHTTP, FrontendHTTPS)
}

func (c *HAProxyController) handleHTTPRedirect(ingress *store.Ingress) error {
	//  Get and validate annotations
	toEnable := false
	annSSLRedirect, _ := c.Store.GetValueFromAnnotations("ssl-redirect", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	annRedirectCode, _ := c.Store.GetValueFromAnnotations("ssl-redirect-code", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	sslRedirectCode, err := strconv.ParseInt(annRedirectCode.Value, 10, 64)
	if err != nil {
		return err
	}
	if annSSLRedirect != nil && annSSLRedirect.Status != DELETED {
		if toEnable, err = utils.GetBoolValue(annSSLRedirect.Value, "ssl-redirect"); err != nil {
			return err
		}
	} else if tlsEnabled(ingress) {
		toEnable = true
	}
	if !toEnable {
		return nil
	}
	// Configure redirection
	match, _ := haproxy.NewMapID(fmt.Sprintf("%d-%d", haproxy.REQ_SSL_REDIRECT, sslRedirectCode))
	for hostname, rule := range ingress.Rules {
		if rule.Status != DELETED {
			for path := range rule.Paths {
				c.cfg.MapFiles.AppendRow(match, hostname+path)
			}
		}
	}
	reqSSLRedirect := rules.ReqSSLRedirect{
		Ingress:      match,
		RedirectCode: sslRedirectCode,
	}
	return c.cfg.HAProxyRules.AddRule(reqSSLRedirect, match, FrontendHTTP)
}

func (c *HAProxyController) handleRateLimiting(ingress *store.Ingress) error {
	//  Get annotations status
	annRateLimitReq, _ := c.Store.GetValueFromAnnotations("rate-limit-requests", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annRateLimitReq == nil {
		return nil
	}
	if ingress.Status == DELETED || annRateLimitReq.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting rate-limit-requests configuration", ingress.Namespace, ingress.Name)
		return nil
	}
	// Validate annotations
	reqsLimit, err := strconv.ParseInt(annRateLimitReq.Value, 10, 64)
	if err != nil {
		return err
	}
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

	// Configure annotation
	logger.Debugf("Ingress %s/%s: Configuring rate-limit-requests annotation", ingress.Namespace, ingress.Name)
	reqsMatch, _ := haproxy.NewMapID(fmt.Sprintf("%d-%d-%d", haproxy.REQ_TRACK, *rateLimitPeriod, reqsLimit))
	trackMatch, _ := haproxy.NewMapID(fmt.Sprintf("%d-%d", haproxy.REQ_RATELIMIT, *rateLimitPeriod))
	tableName := fmt.Sprintf("RateLimit-%d", *rateLimitPeriod)
	for hostname, rule := range ingress.Rules {
		if rule.Status != DELETED {
			for path := range rule.Paths {
				c.cfg.MapFiles.AppendRow(reqsMatch, hostname+path)
				c.cfg.MapFiles.AppendRow(trackMatch, hostname+path)
			}
		}
	}
	rateLimitTables[tableName] = rateLimitTable{
		size:   rateLimitSize,
		period: rateLimitPeriod,
	}
	reqTrack := rules.ReqTrack{
		Ingress:   trackMatch,
		TableName: tableName,
		TrackKey:  "src",
	}
	err = c.cfg.HAProxyRules.AddRule(reqTrack, trackMatch, FrontendHTTP, FrontendHTTPS)
	if err != nil {
		return err
	}
	reqRateLimit := rules.ReqRateLimit{
		Ingress:        reqsMatch,
		ReqsLimit:      reqsLimit,
		DenyStatusCode: rateLimitCode,
	}
	return c.cfg.HAProxyRules.AddRule(reqRateLimit, reqsMatch, FrontendHTTP, FrontendHTTPS)
}

func (c *HAProxyController) handleRequestCapture(ingress *store.Ingress) error {
	//  Get annotation status
	annReqCapture, _ := c.Store.GetValueFromAnnotations("request-capture", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annReqCapture == nil {
		return nil
	}
	if ingress.Status == DELETED || annReqCapture.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting request-capture configuration", ingress.Namespace, ingress.Name)
		return nil
	}
	//  Validate annotation
	annCaptureLen, _ := c.Store.GetValueFromAnnotations("request-capture-len", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	captureLen, err := strconv.ParseInt(annCaptureLen.Value, 10, 64)
	if err != nil {
		return err
	}

	// Configure annotation
	for _, sample := range strings.Split(annReqCapture.Value, "\n") {
		logger.Debugf("Ingress %s/%s: Configuring request capture for '%s'", ingress.Namespace, ingress.Name, sample)
		if sample == "" {
			continue
		}
		match, _ := haproxy.NewMapID(fmt.Sprintf("%d-%s-%d", haproxy.REQ_CAPTURE, sample, captureLen))
		for hostname, rule := range ingress.Rules {
			if rule.Status != DELETED {
				for path := range rule.Paths {
					c.cfg.MapFiles.AppendRow(match, hostname+path)
				}
			}
		}
		reqCapture := rules.ReqCapture{
			Ingress:    match,
			Expression: sample,
			CaptureLen: captureLen,
		}
		err = c.cfg.HAProxyRules.AddRule(reqCapture, match, FrontendHTTP, FrontendHTTPS)
	}

	// TODO handle stacking error
	return err
}

func (c *HAProxyController) handleRequestSetHdr(ingress *store.Ingress) error {
	//  Get annotation status
	annReqSetHdr, _ := c.Store.GetValueFromAnnotations("request-set-header", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annReqSetHdr == nil {
		return nil
	}
	if ingress.Status == DELETED || annReqSetHdr.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting request-set-header configuration", ingress.Namespace, ingress.Name)
		return nil
	}
	// Configure annotation
	var err error
	for _, param := range strings.Split(annReqSetHdr.Value, "\n") {
		parts := strings.Fields(param)
		if len(parts) != 2 {
			logger.Errorf("incorrect value '%s' in request-set-header annotation", param)
			continue
		}
		logger.Debugf("Ingress %s/%s: Configuring request set '%s' header ", ingress.Namespace, ingress.Name, param)
		match, _ := haproxy.NewMapID(fmt.Sprintf("%d-%s-%s", haproxy.REQ_SET_HEADER, parts[0], parts[1]))
		for hostname, rule := range ingress.Rules {
			if rule.Status != DELETED {
				for path := range rule.Paths {
					c.cfg.MapFiles.AppendRow(match, hostname+path)
				}
			}
		}
		reqSetHdr := rules.SetHdr{
			Ingress:   match,
			HdrName:   parts[0],
			HdrFormat: parts[1],
		}
		err = c.cfg.HAProxyRules.AddRule(reqSetHdr, match, FrontendHTTP, FrontendHTTPS)
	}
	//TODO: handle stacking errors
	return err
}

func (c *HAProxyController) handleRequestSetHost(ingress *store.Ingress) error {
	//  Get annotation status
	annSetHost, _ := c.Store.GetValueFromAnnotations("set-host", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annSetHost == nil {
		return nil
	}
	if ingress.Status == DELETED || annSetHost.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting request-set-host configuration", ingress.Namespace, ingress.Name)
		return nil
	}
	// Configure annotation
	logger.Debugf("Ingress %s/%s: Configuring request-set-host", ingress.Namespace, ingress.Name)
	match, _ := haproxy.NewMapID(fmt.Sprintf("%d-%s", haproxy.REQ_SET_HOST, annSetHost.Value))
	for hostname, rule := range ingress.Rules {
		if rule.Status != DELETED {
			for path := range rule.Paths {
				c.cfg.MapFiles.AppendRow(match, hostname+path)
			}
		}
	}
	reqSetHost := rules.SetHdr{
		Ingress:   match,
		HdrName:   "Host",
		HdrFormat: annSetHost.Value,
	}
	return c.cfg.HAProxyRules.AddRule(reqSetHost, match, FrontendHTTP, FrontendHTTPS)
}

func (c *HAProxyController) handleRequestPathRewrite(ingress *store.Ingress) error {
	//  Get annotation status
	annPathRewrite, _ := c.Store.GetValueFromAnnotations("path-rewrite", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annPathRewrite == nil {
		return nil
	}
	if ingress.Status == DELETED || annPathRewrite.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting path-rewrite configuration", ingress.Namespace, ingress.Name)
		return nil
	}
	// Configure annotation
	logger.Debugf("Ingress %s/%s: Configuring path-rewrite", ingress.Namespace, ingress.Name)
	parts := strings.Fields(strings.TrimSpace(annPathRewrite.Value))
	match, _ := haproxy.NewMapID(fmt.Sprintf("%d-%s", haproxy.REQ_PATH_REWRITE, annPathRewrite.Value))
	for hostname, rule := range ingress.Rules {
		if rule.Status != DELETED {
			for path := range rule.Paths {
				c.cfg.MapFiles.AppendRow(match, hostname+path)
			}
		}
	}

	var pathReWrite haproxy.Rule
	switch len(parts) {
	case 1:
		pathReWrite = rules.ReqPathRewrite{
			Ingress:   match,
			PathMatch: "(.*)",
			PathFmt:   parts[0],
		}
	case 2:
		pathReWrite = rules.ReqPathRewrite{
			Ingress:   match,
			PathMatch: parts[0],
			PathFmt:   parts[1],
		}
	default:
		return fmt.Errorf("incorrect value '%s', path-rewrite takes 1 or 2 params ", annPathRewrite.Value)
	}
	return c.cfg.HAProxyRules.AddRule(pathReWrite, match, FrontendHTTP, FrontendHTTPS)
}

func (c *HAProxyController) handleResponseSetHdr(ingress *store.Ingress) error {
	//  Get annotation status
	annResSetHdr, _ := c.Store.GetValueFromAnnotations("response-set-header", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annResSetHdr == nil {
		return nil
	}
	if ingress.Status == DELETED || annResSetHdr.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting response-set-header configuration", ingress.Namespace, ingress.Name)
		return nil
	}
	// Configure annotation
	var err error
	for _, param := range strings.Split(annResSetHdr.Value, "\n") {
		parts := strings.Fields(param)
		if len(parts) != 2 {
			logger.Errorf("incorrect value '%s' in response-set-header annotation", param)
			continue
		}
		logger.Debugf("Ingress %s/%s: Configuring reponse set '%s' header ", ingress.Namespace, ingress.Name, param)
		match, _ := haproxy.NewMapID(fmt.Sprintf("%d-%s-%s", haproxy.RES_SET_HEADER, parts[0], parts[1]))
		for hostname, rule := range ingress.Rules {
			if rule.Status != DELETED {
				for path := range rule.Paths {
					c.cfg.MapFiles.AppendRow(match, hostname+path)
				}
			}
		}
		resSetHdr := rules.SetHdr{
			Ingress:   match,
			HdrName:   parts[0],
			HdrFormat: parts[1],
			Response:  true,
		}
		err = c.cfg.HAProxyRules.AddRule(resSetHdr, match, FrontendHTTP, FrontendHTTPS)
	}
	//TODO: handle stacking errors
	return err
}

func (c *HAProxyController) handleWhitelisting(ingress *store.Ingress) error {
	//  Get annotation status
	annWhitelist, _ := c.Store.GetValueFromAnnotations("whitelist", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annWhitelist == nil {
		return nil
	}
	if ingress.Status == DELETED || annWhitelist.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting whitelist configuration", ingress.Namespace, ingress.Name)
		return nil
	}
	// Validate annotation
	ips, _ := haproxy.NewMapID(annWhitelist.Value)
	for _, address := range strings.Split(annWhitelist.Value, ",") {
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				logger.Errorf("incorrect address '%s' in whitelist annotation in ingress '%s'", address, ingress.Name)
				continue
			}
		}
		c.cfg.MapFiles.AppendRow(ips, address)
	}
	// Configure annotation
	logger.Debugf("Ingress %s/%s: Configuring whitelist annotation", ingress.Namespace, ingress.Name)
	match, _ := haproxy.NewMapID(fmt.Sprintf("%d-%s", haproxy.REQ_DENY, annWhitelist.Value))
	for hostname, rule := range ingress.Rules {
		if rule.Status != DELETED {
			for path := range rule.Paths {
				c.cfg.MapFiles.AppendRow(match, hostname+path)
			}
		}
	}
	reqWhitelist := rules.ReqDeny{
		Ingress:   match,
		SrcIPs:    ips,
		Whitelist: true,
	}
	return c.cfg.HAProxyRules.AddRule(reqWhitelist, match, FrontendHTTP, FrontendHTTPS)
}

func tlsEnabled(ingress *store.Ingress) bool {
	for _, tls := range ingress.TLS {
		if tls.Status != DELETED {
			return true
		}
	}
	return false
}
