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

var rateLimitTables []string

func (c *HAProxyController) handleIngressAnnotations(ingress *store.Ingress) {
	c.handleRateLimiting(ingress)
	c.handleRequestCapture(ingress)
	c.handleRequestPathRewrite(ingress)
	c.handleRequestSetHost(ingress)
	c.handleRequestSetHdr(ingress)
	c.handleResponseSetHdr(ingress)
	c.handleBlacklisting(ingress)
	c.handleHTTPBasicAuth(ingress)
	c.handleWhitelisting(ingress)
	c.handleHTTPSRedirect(ingress)
	c.handleHostRedirect(ingress)
}

func (c *HAProxyController) handleBlacklisting(ingress *store.Ingress) {
	//  Get annotation status
	annBlacklist, _ := c.Store.GetValueFromAnnotations("blacklist", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annBlacklist == nil {
		return
	}
	if annBlacklist.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting blacklist configuration", ingress.Namespace, ingress.Name)
		return
	}
	// Validate annotation
	mapName := "blacklist-" + strconv.Itoa(int(utils.Hash([]byte(annBlacklist.Value))))
	for _, address := range strings.Split(annBlacklist.Value, ",") {
		address = strings.TrimSpace(address)
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				logger.Errorf("incorrect address '%s' in blacklist annotation in ingress '%s'", address, ingress.Name)
				continue
			}
		}
		c.cfg.MapFiles.AppendRow(mapName, address)
	}
	// Configure annotation
	logger.Debugf("Ingress %s/%s: Configuring blacklist annotation", ingress.Namespace, ingress.Name)
	reqBlackList := rules.ReqDeny{
		SrcIPsMap: mapName,
	}
	logger.Error(c.cfg.HAProxyRules.AddRule(reqBlackList, &ingress.Name, FrontendHTTP, FrontendHTTPS))
}

func (c *HAProxyController) handleHostRedirect(ingress *store.Ingress) {
	//  Get and validate annotations
	annDomainRedirect, _ := c.Store.GetValueFromAnnotations("request-redirect", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	annDomainRedirectCode, _ := c.Store.GetValueFromAnnotations("request-redirect-code", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	domainRedirectCode, err := strconv.ParseInt(annDomainRedirectCode.Value, 10, 64)
	if err != nil {
		logger.Error(err)
		return
	}
	if annDomainRedirect == nil || annDomainRedirect.Status == DELETED {
		return
	}
	// Configure redirection
	reqDomainRedirect := rules.RequestRedirect{
		RedirectCode: domainRedirectCode,
		Host:         annDomainRedirect.Value,
	}
	logger.Error(c.cfg.HAProxyRules.AddRule(reqDomainRedirect, &ingress.Name, FrontendHTTP))
	reqDomainRedirect.SSLRequest = true
	logger.Error(c.cfg.HAProxyRules.AddRule(reqDomainRedirect, &ingress.Name, FrontendHTTPS))

}
func (c *HAProxyController) handleHTTPSRedirect(ingress *store.Ingress) {
	//  Get and validate annotations
	toEnable := false
	annSSLRedirect, _ := c.Store.GetValueFromAnnotations("ssl-redirect", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	annSSLRedirectPort, _ := c.Store.GetValueFromAnnotations("ssl-redirect-port", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	annRedirectCode, _ := c.Store.GetValueFromAnnotations("ssl-redirect-code", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	sslRedirectCode, err := strconv.ParseInt(annRedirectCode.Value, 10, 64)
	if err != nil {
		logger.Error(err)
		return
	}
	if annSSLRedirect != nil && annSSLRedirect.Status != DELETED {
		if toEnable, err = utils.GetBoolValue(annSSLRedirect.Value, "ssl-redirect"); err != nil {
			logger.Error(err)
			return
		}
	} else if tlsEnabled(ingress) {
		toEnable = true
	}
	if !toEnable {
		return
	}
	sslRedirectPort, err := strconv.Atoi(annSSLRedirectPort.Value)
	if err != nil {
		logger.Error(err)
		return
	}
	// Configure redirection
	reqSSLRedirect := rules.RequestRedirect{
		RedirectCode: sslRedirectCode,
		RedirectPort: sslRedirectPort,
		SSLRedirect:  true,
	}
	logger.Error(c.cfg.HAProxyRules.AddRule(reqSSLRedirect, &ingress.Name, FrontendHTTP))
}

func (c *HAProxyController) handleRateLimiting(ingress *store.Ingress) {
	//  Get annotations status
	annRateLimitReq, _ := c.Store.GetValueFromAnnotations("rate-limit-requests", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annRateLimitReq == nil {
		return
	}
	if annRateLimitReq.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting rate-limit-requests configuration", ingress.Namespace, ingress.Name)
		return
	}
	// Validate annotations
	reqsLimit, err := strconv.ParseInt(annRateLimitReq.Value, 10, 64)
	if err != nil {
		logger.Error(err)
		return
	}
	annRateLimitPeriod, _ := c.Store.GetValueFromAnnotations("rate-limit-period", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	rateLimitPeriod, err := utils.ParseTime(annRateLimitPeriod.Value)
	if err != nil {
		logger.Error(err)
		return
	}
	annRateLimitSize, _ := c.Store.GetValueFromAnnotations("rate-limit-size", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	rateLimitSize := misc.ParseSize(annRateLimitSize.Value)

	annRateLimitCode, _ := c.Store.GetValueFromAnnotations("rate-limit-status-code", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	rateLimitCode, err := utils.ParseInt(annRateLimitCode.Value)
	if err != nil {
		logger.Error(err)
		return
	}

	// Configure annotation
	logger.Debugf("Ingress %s/%s: Configuring rate-limit-requests annotation", ingress.Namespace, ingress.Name)
	tableName := fmt.Sprintf("RateLimit-%d", *rateLimitPeriod)
	rateLimitTables = append(rateLimitTables, tableName)
	reqTrack := rules.ReqTrack{
		TableName:   tableName,
		TableSize:   rateLimitSize,
		TablePeriod: rateLimitPeriod,
		TrackKey:    "src",
	}
	reqRateLimit := rules.ReqRateLimit{
		TableName:      tableName,
		ReqsLimit:      reqsLimit,
		DenyStatusCode: rateLimitCode,
	}
	logger.Error(c.cfg.HAProxyRules.AddRule(reqTrack, &ingress.Name, FrontendHTTP, FrontendHTTPS))
	logger.Error(c.cfg.HAProxyRules.AddRule(reqRateLimit, &ingress.Name, FrontendHTTP, FrontendHTTPS))
}

func (c *HAProxyController) handleRequestCapture(ingress *store.Ingress) {
	//  Get annotation status
	annReqCapture, _ := c.Store.GetValueFromAnnotations("request-capture", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annReqCapture == nil {
		return
	}
	if annReqCapture.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting request-capture configuration", ingress.Namespace, ingress.Name)
		return
	}
	//  Validate annotation
	annCaptureLen, _ := c.Store.GetValueFromAnnotations("request-capture-len", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	captureLen, err := strconv.ParseInt(annCaptureLen.Value, 10, 64)
	if err != nil {
		logger.Error(err)
		return
	}

	// Configure annotation
	for _, sample := range strings.Split(annReqCapture.Value, "\n") {
		logger.Debugf("Ingress %s/%s: Configuring request capture for '%s'", ingress.Namespace, ingress.Name, sample)
		if sample == "" {
			continue
		}
		reqCapture := rules.ReqCapture{
			Expression: sample,
			CaptureLen: captureLen,
		}
		logger.Error(c.cfg.HAProxyRules.AddRule(reqCapture, &ingress.Name, FrontendHTTP, FrontendHTTPS))
	}
}

func (c *HAProxyController) handleRequestSetHdr(ingress *store.Ingress) {
	//  Get annotation status
	annReqSetHdr, _ := c.Store.GetValueFromAnnotations("request-set-header", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annReqSetHdr == nil {
		return
	}
	if annReqSetHdr.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting request-set-header configuration", ingress.Namespace, ingress.Name)
		return
	}
	// Configure annotation
	for _, param := range strings.Split(annReqSetHdr.Value, "\n") {
		parts := strings.Fields(param)
		if len(parts) != 2 {
			logger.Errorf("incorrect value '%s' in request-set-header annotation", param)
			continue
		}
		logger.Debugf("Ingress %s/%s: Configuring request set '%s' header ", ingress.Namespace, ingress.Name, param)
		reqSetHdr := rules.SetHdr{
			HdrName:   parts[0],
			HdrFormat: parts[1],
		}
		logger.Error(c.cfg.HAProxyRules.AddRule(reqSetHdr, &ingress.Name, FrontendHTTP, FrontendHTTPS))
	}
}

func (c *HAProxyController) handleRequestSetHost(ingress *store.Ingress) {
	//  Get annotation status
	annSetHost, _ := c.Store.GetValueFromAnnotations("set-host", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annSetHost == nil {
		return
	}
	if annSetHost.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting request-set-host configuration", ingress.Namespace, ingress.Name)
		return
	}
	// Configure annotation
	logger.Debugf("Ingress %s/%s: Configuring request-set-host", ingress.Namespace, ingress.Name)
	reqSetHost := rules.SetHdr{
		HdrName:   "Host",
		HdrFormat: annSetHost.Value,
	}
	logger.Error(c.cfg.HAProxyRules.AddRule(reqSetHost, &ingress.Name, FrontendHTTP, FrontendHTTPS))
}

func (c *HAProxyController) handleRequestPathRewrite(ingress *store.Ingress) {
	//  Get annotation status
	annPathRewrite, _ := c.Store.GetValueFromAnnotations("path-rewrite", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annPathRewrite == nil {
		return
	}
	if annPathRewrite.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting path-rewrite configuration", ingress.Namespace, ingress.Name)
		return
	}
	// Configure annotation
	logger.Debugf("Ingress %s/%s: Configuring path-rewrite", ingress.Namespace, ingress.Name)
	parts := strings.Fields(strings.TrimSpace(annPathRewrite.Value))

	var reqPathReWrite haproxy.Rule
	switch len(parts) {
	case 1:
		reqPathReWrite = rules.ReqPathRewrite{
			PathMatch: "(.*)",
			PathFmt:   parts[0],
		}
	case 2:
		reqPathReWrite = rules.ReqPathRewrite{
			PathMatch: parts[0],
			PathFmt:   parts[1],
		}
	default:
		logger.Errorf("incorrect value '%s', path-rewrite takes 1 or 2 params ", annPathRewrite.Value)
		return
	}
	logger.Error(c.cfg.HAProxyRules.AddRule(reqPathReWrite, &ingress.Name, FrontendHTTP, FrontendHTTPS))
}

func (c *HAProxyController) handleResponseSetHdr(ingress *store.Ingress) {
	//  Get annotation status
	annResSetHdr, _ := c.Store.GetValueFromAnnotations("response-set-header", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annResSetHdr == nil {
		return
	}
	if annResSetHdr.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting response-set-header configuration", ingress.Namespace, ingress.Name)
		return
	}
	// Configure annotation
	for _, param := range strings.Split(annResSetHdr.Value, "\n") {
		parts := strings.Fields(param)
		if len(parts) != 2 {
			logger.Errorf("incorrect value '%s' in response-set-header annotation", param)
			continue
		}
		logger.Debugf("Ingress %s/%s: Configuring response set '%s' header ", ingress.Namespace, ingress.Name, param)
		resSetHdr := rules.SetHdr{
			HdrName:   parts[0],
			HdrFormat: parts[1],
			Response:  true,
		}
		logger.Error(c.cfg.HAProxyRules.AddRule(resSetHdr, &ingress.Name, FrontendHTTP, FrontendHTTPS))

	}
}

func (c *HAProxyController) handleWhitelisting(ingress *store.Ingress) {
	//  Get annotation status
	annWhitelist, _ := c.Store.GetValueFromAnnotations("whitelist", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if annWhitelist == nil {
		return
	}
	if annWhitelist.Status == DELETED {
		logger.Debugf("Ingress %s/%s: Deleting whitelist configuration", ingress.Namespace, ingress.Name)
		return
	}
	// Validate annotation
	mapName := "whitelist-" + strconv.Itoa(int(utils.Hash([]byte(annWhitelist.Value))))
	for _, address := range strings.Split(annWhitelist.Value, ",") {
		address = strings.TrimSpace(address)
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				logger.Errorf("incorrect address '%s' in whitelist annotation in ingress '%s'", address, ingress.Name)
				continue
			}
		}
		c.cfg.MapFiles.AppendRow(mapName, address)
	}
	// Configure annotation
	logger.Debugf("Ingress %s/%s: Configuring whitelist annotation", ingress.Namespace, ingress.Name)
	reqWhitelist := rules.ReqDeny{
		SrcIPsMap: mapName,
		Whitelist: true,
	}
	logger.Error(c.cfg.HAProxyRules.AddRule(reqWhitelist, &ingress.Name, FrontendHTTP, FrontendHTTPS))
}

func (c *HAProxyController) handleHTTPBasicAuth(ingress *store.Ingress) {
	userListName := fmt.Sprintf("%s-%s", ingress.Namespace, ingress.Name)
	authType, _ := c.Store.GetValueFromAnnotations("auth-type", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	authSecret, _ := c.Store.GetValueFromAnnotations("auth-secret", ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	switch {
	case authType == nil:
		return
	case authType.Value != "basic-auth":
		logger.Errorf("Ingress %s/%s: incorrect auth-type value '%s'. Only 'basic-auth' value is currently supported", ingress.Namespace, ingress.Name, authType.Value)
	case authType.Status == DELETED:
		logger.Debugf("Ingress %s/%s: Deleting HTTP Basic Authentication", ingress.Namespace, ingress.Name)
		logger.Error(c.Client.UserListDeleteByGroup(userListName))
		return
	case authSecret == nil || authSecret.Status == DELETED:
		logger.Warningf("Ingress %s/%s: auth-type annotation active but no auth-secret provided. Service won't be accessible", ingress.Namespace, ingress.Name)
	}

	// Parsing secret
	credentials := make(map[string][]byte)
	if authSecret != nil {
		if secret, err := c.Store.FetchSecret(authSecret.Value, ingress.Namespace); secret == nil {
			logger.Warningf("Ingress %s/%s: %s", ingress.Namespace, ingress.Name, err)
		} else {
			if secret.Status == DELETED {
				logger.Warningf("Ingress %s/%s: Secret %s deleted but auth-type annotaiton still active", ingress.Namespace, ingress.Name, secret.Name)
			}
			for u, pwd := range secret.Data {
				if pwd[len(pwd)-1] == '\n' {
					logger.Warningf("Ingress %s/%s: basic-auth: password for user %s ends with '\\n'. Ignoring last character.", ingress.Namespace, ingress.Name, u)
					pwd = pwd[:len(pwd)-1]
				}
				credentials[u] = pwd
			}
		}
	}
	// Configuring annotation
	var errors utils.Errors
	errors.Add(
		c.Client.UserListDeleteByGroup(userListName),
		c.Client.UserListCreateByGroup(userListName, credentials))
	if errors.Result() != nil {
		logger.Errorf("Ingress %s/%s: Cannot create userlist for basic-auth, %s", ingress.Namespace, ingress.Name, errors.Result())
		return
	}

	// Adding HAProxy Rule
	logger.Debugf("Ingress %s/%s: Configuring basic-auth annotation", ingress.Namespace, ingress.Name)
	reqBasicAuth := rules.ReqBasicAuth{
		Data:      credentials,
		AuthGroup: userListName,
	}
	logger.Error(c.cfg.HAProxyRules.AddRule(reqBasicAuth, &ingress.Name, FrontendHTTP, FrontendHTTPS))
}

func tlsEnabled(ingress *store.Ingress) bool {
	for _, tls := range ingress.TLS {
		if tls.Status != DELETED {
			return true
		}
	}
	return false
}
