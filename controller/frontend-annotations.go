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

	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

func (c *HAProxyController) handleIngressAnnotations(ingress *store.Ingress) []haproxy.RuleID {
	result := haproxy.Rules{}
	c.handleSourceIPHeader(ingress, &result)
	c.handleBlacklisting(ingress, &result)
	c.handleWhitelisting(ingress, &result)
	c.handleRequestRateLimiting(ingress, &result)
	c.handleRequestBasicAuth(ingress, &result)
	c.handleRequestHostRedirect(ingress, &result)
	c.handleRequestHTTPSRedirect(ingress, &result)
	c.handleRequestCapture(ingress, &result)
	c.handleRequestPathRewrite(ingress, &result)
	c.handleRequestSetHost(ingress, &result)
	c.handleRequestSetHdr(ingress, &result)
	c.handleResponseSetHdr(ingress, &result)
	c.handleResponseCors(ingress, &result)

	ids := []haproxy.RuleID{}
	frontends := []string{c.Cfg.FrontHTTP, c.Cfg.FrontHTTPS}
	for _, rule := range result {
		switch rule.GetType() {
		case haproxy.REQ_REDIRECT:
			redirRule := rule.(rules.RequestRedirect)
			if redirRule.SSLRedirect {
				frontends = []string{c.Cfg.FrontHTTP}
			} else {
				frontends = []string{c.Cfg.FrontHTTP, c.Cfg.FrontHTTPS}
			}
		case haproxy.REQ_DENY, haproxy.REQ_CAPTURE:
			if c.sslPassthroughEnabled(ingress, nil) {
				frontends = []string{c.Cfg.FrontHTTP, c.Cfg.FrontSSL}
			}
		}
		for _, frontend := range frontends {
			logger.Error(c.Cfg.HAProxyRules.AddRule(rule, true, frontend))
		}
		ids = append(ids, haproxy.GetID(rule))
	}
	return ids
}

func (c *HAProxyController) handleSourceIPHeader(ingress *store.Ingress, result *haproxy.Rules) {
	srcIPHeader := annotations.GetValue("src-ip-header", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)

	if srcIPHeader == "" || len(srcIPHeader) == 0 {
		return
	}
	logger.Tracef("Ingress %s/%s: Configuring Source IP annotation", ingress.Namespace, ingress.Name)
	reqSetSrc := rules.ReqSetSrc{
		HeaderName: srcIPHeader,
	}
	result.Add(reqSetSrc)
}

func (c *HAProxyController) handleBlacklisting(ingress *store.Ingress, result *haproxy.Rules) {
	//  Get annotation status
	annBlacklist := annotations.GetValue("blacklist", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annBlacklist == "" {
		return
	}
	// Validate annotation
	mapName := "blacklist-" + utils.Hash([]byte(annBlacklist))
	if !c.Cfg.MapFiles.Exists(mapName) {
		for _, address := range strings.Split(annBlacklist, ",") {
			address = strings.TrimSpace(address)
			if ip := net.ParseIP(address); ip == nil {
				if _, _, err := net.ParseCIDR(address); err != nil {
					logger.Errorf("incorrect address '%s' in blacklist annotation in ingress '%s'", address, ingress.Name)
					continue
				}
			}
			c.Cfg.MapFiles.AppendRow(mapName, address)
		}
	}
	// Configure annotation
	logger.Tracef("Ingress %s/%s: Configuring blacklist annotation", ingress.Namespace, ingress.Name)
	reqBlackList := rules.ReqDeny{
		SrcIPsMap: mapName,
	}
	result.Add(reqBlackList)
}

func (c *HAProxyController) handleWhitelisting(ingress *store.Ingress, result *haproxy.Rules) {
	//  Get annotation status
	annWhitelist := annotations.GetValue("whitelist", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annWhitelist == "" {
		return
	}
	// Validate annotation
	mapName := "whitelist-" + utils.Hash([]byte(annWhitelist))
	if !c.Cfg.MapFiles.Exists(mapName) {
		for _, address := range strings.Split(annWhitelist, ",") {
			address = strings.TrimSpace(address)
			if ip := net.ParseIP(address); ip == nil {
				if _, _, err := net.ParseCIDR(address); err != nil {
					logger.Errorf("incorrect address '%s' in whitelist annotation in ingress '%s'", address, ingress.Name)
					continue
				}
			}
			c.Cfg.MapFiles.AppendRow(mapName, address)
		}
	}
	// Configure annotation
	logger.Tracef("Ingress %s/%s: Configuring whitelist annotation", ingress.Namespace, ingress.Name)
	reqWhitelist := rules.ReqDeny{
		SrcIPsMap: mapName,
		Whitelist: true,
	}
	result.Add(reqWhitelist)
}

func (c *HAProxyController) handleRequestRateLimiting(ingress *store.Ingress, result *haproxy.Rules) {
	//  Get annotations status
	annRateLimitReq := annotations.GetValue("rate-limit-requests", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annRateLimitReq == "" {
		return
	}
	// Validate annotations
	reqsLimit, err := strconv.ParseInt(annRateLimitReq, 10, 64)
	if err != nil {
		logger.Error(err)
		return
	}
	annRateLimitPeriod := annotations.GetValue("rate-limit-period", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	rateLimitPeriod, err := utils.ParseTime(annRateLimitPeriod)
	if err != nil {
		logger.Error(err)
		return
	}
	annRateLimitSize := annotations.GetValue("rate-limit-size", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	rateLimitSize := misc.ParseSize(annRateLimitSize)

	annRateLimitCode := annotations.GetValue("rate-limit-status-code", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	rateLimitCode, err := utils.ParseInt(annRateLimitCode)
	if err != nil {
		logger.Error(err)
		return
	}

	// Configure annotation
	logger.Tracef("Ingress %s/%s: Configuring rate-limit-requests annotation", ingress.Namespace, ingress.Name)
	tableName := fmt.Sprintf("RateLimit-%d", *rateLimitPeriod)
	c.Cfg.RateLimitTables = append(c.Cfg.RateLimitTables, tableName)
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
	result.Add(reqTrack)
	result.Add(reqRateLimit)
}

func (c *HAProxyController) handleRequestBasicAuth(ingress *store.Ingress, result *haproxy.Rules) {
	userListName := fmt.Sprintf("%s-%s", ingress.Namespace, ingress.Name)
	authType := annotations.GetValue("auth-type", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	authSecret := annotations.GetValue("auth-secret", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	authRealm := annotations.GetValue("auth-realm", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	switch {
	case authType == "":
		if ok, _ := c.Client.UserListExistsByGroup(userListName); ok {
			logger.Tracef("Ingress %s/%s: Deleting HTTP Basic Authentication", ingress.Namespace, ingress.Name)
			logger.Error(c.Client.UserListDeleteByGroup(userListName))
		}
		return
	case authType != "basic-auth":
		logger.Errorf("Ingress %s/%s: incorrect auth-type value '%s'. Only 'basic-auth' value is currently supported", ingress.Namespace, ingress.Name, authType)
	case authSecret == "":
		logger.Warningf("Ingress %s/%s: auth-type annotation active but no auth-secret provided. Service won't be accessible", ingress.Namespace, ingress.Name)
	}

	// Parsing secret
	credentials := make(map[string][]byte)
	if authSecret != "" {
		if secret, err := c.Store.FetchSecret(authSecret, ingress.Namespace); secret == nil {
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

	realm := "Protected-Content"
	if authRealm != "" {
		realm = strings.ReplaceAll(authRealm, " ", "-")
	}
	// Adding HAProxy Rule
	logger.Tracef("Ingress %s/%s: Configuring basic-auth annotation", ingress.Namespace, ingress.Name)
	reqBasicAuth := rules.ReqBasicAuth{
		Data:      credentials,
		AuthRealm: realm,
		AuthGroup: userListName,
	}
	result.Add(reqBasicAuth)
}

func (c *HAProxyController) handleRequestHostRedirect(ingress *store.Ingress, result *haproxy.Rules) {
	//  Get and validate annotations
	annDomainRedirect := annotations.GetValue("request-redirect", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	annDomainRedirectCode := annotations.GetValue("request-redirect-code", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	domainRedirectCode, err := strconv.ParseInt(annDomainRedirectCode, 10, 64)
	if err != nil {
		logger.Error(err)
		return
	}
	if annDomainRedirect == "" {
		return
	}
	// Configure redirection
	reqHTTPDomRedir := rules.RequestRedirect{
		RedirectCode: domainRedirectCode,
		Host:         annDomainRedirect,
	}
	reqHTTPSDomRedir := reqHTTPDomRedir
	reqHTTPSDomRedir.SSLRedirect = true
	result.Add(reqHTTPDomRedir)
	result.Add(reqHTTPSDomRedir)
}

func (c *HAProxyController) handleRequestHTTPSRedirect(ingress *store.Ingress, result *haproxy.Rules) {
	//  Get and validate annotations
	toEnable := false
	annSSLRedirect := annotations.GetValue("ssl-redirect", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	annSSLRedirectPort := annotations.GetValue("ssl-redirect-port", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	annRedirectCode := annotations.GetValue("ssl-redirect-code", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	sslRedirectCode, err := strconv.ParseInt(annRedirectCode, 10, 64)
	if err != nil {
		logger.Error(err)
		return
	}
	if annSSLRedirect != "" {
		if toEnable, err = utils.GetBoolValue(annSSLRedirect, "ssl-redirect"); err != nil {
			logger.Error(err)
			return
		}
	} else if tlsEnabled(ingress) {
		toEnable = true
	}
	if !toEnable {
		return
	}
	sslRedirectPort, err := strconv.Atoi(annSSLRedirectPort)
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
	result.Add(reqSSLRedirect)
}

func (c *HAProxyController) handleRequestCapture(ingress *store.Ingress, result *haproxy.Rules) {
	//  Get annotation status
	annReqCapture := annotations.GetValue("request-capture", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annReqCapture == "" {
		return
	}
	//  Validate annotation
	annCaptureLen := annotations.GetValue("request-capture-len", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	captureLen, err := strconv.ParseInt(annCaptureLen, 10, 64)
	if err != nil {
		logger.Error(err)
		return
	}

	// Configure annotation
	for _, sample := range strings.Split(annReqCapture, "\n") {
		logger.Tracef("Ingress %s/%s: Configuring request capture for '%s'", ingress.Namespace, ingress.Name, sample)
		if sample == "" {
			continue
		}
		reqCapture := rules.ReqCapture{
			Expression: sample,
			CaptureLen: captureLen,
		}
		result.Add(reqCapture)
	}
}

func (c *HAProxyController) handleRequestSetHost(ingress *store.Ingress, result *haproxy.Rules) {
	//  Get annotation status
	annSetHost := annotations.GetValue("set-host", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annSetHost == "" {
		return
	}
	// Configure annotation
	logger.Tracef("Ingress %s/%s: Configuring request-set-host", ingress.Namespace, ingress.Name)
	reqSetHost := rules.SetHdr{
		HdrName:   "Host",
		HdrFormat: annSetHost,
	}
	result.Add(reqSetHost)
}

func (c *HAProxyController) handleRequestPathRewrite(ingress *store.Ingress, result *haproxy.Rules) {
	//  Get annotation status
	annPathRewrite := annotations.GetValue("path-rewrite", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annPathRewrite == "" {
		return
	}
	// Configure annotation
	logger.Tracef("Ingress %s/%s: Configuring path-rewrite", ingress.Namespace, ingress.Name)
	parts := strings.Fields(strings.TrimSpace(annPathRewrite))

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
		logger.Errorf("incorrect value '%s', path-rewrite takes 1 or 2 params ", annPathRewrite)
		return
	}
	result.Add(reqPathReWrite)
}

func (c *HAProxyController) handleRequestSetHdr(ingress *store.Ingress, result *haproxy.Rules) {
	//  Get annotation status
	annReqSetHdr := annotations.GetValue("request-set-header", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annReqSetHdr == "" {
		return
	}
	// Configure annotation
	for _, param := range strings.Split(annReqSetHdr, "\n") {
		if param == "" {
			continue
		}
		indexSpace := strings.IndexByte(param, ' ')
		if indexSpace == -1 {
			logger.Errorf("incorrect value '%s' in request-set-header annotation", param)
			continue
		}
		logger.Tracef("Ingress %s/%s: Configuring request set '%s' header ", ingress.Namespace, ingress.Name, param)
		reqSetHdr := rules.SetHdr{
			HdrName:   param[:indexSpace],
			HdrFormat: "\"" + param[indexSpace+1:] + "\"",
		}
		result.Add(reqSetHdr)
	}
}

func (c *HAProxyController) handleResponseSetHdr(ingress *store.Ingress, result *haproxy.Rules) {
	//  Get annotation status
	annResSetHdr := annotations.GetValue("response-set-header", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annResSetHdr == "" {
		return
	}
	// Configure annotation
	for _, param := range strings.Split(annResSetHdr, "\n") {
		if param == "" {
			continue
		}
		indexSpace := strings.IndexByte(param, ' ')
		if indexSpace == -1 {
			logger.Errorf("incorrect value '%s' in response-set-header annotation", param)
			continue
		}
		logger.Tracef("Ingress %s/%s: Configuring response set '%s' header ", ingress.Namespace, ingress.Name, param)
		resSetHdr := rules.SetHdr{
			HdrName:   param[:indexSpace],
			HdrFormat: "\"" + param[indexSpace+1:] + "\"",
			Response:  true,
		}
		result.Add(resSetHdr)
	}
}

func (c *HAProxyController) handleResponseCors(ingress *store.Ingress, result *haproxy.Rules) {
	annotation := annotations.GetValue("cors-enable", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annotation == "" {
		return
	}
	enabled, err := utils.GetBoolValue(annotation, "cors-enable")
	if err != nil {
		logger.Error(err)
		return
	}
	if !enabled {
		logger.Tracef("Ingress %s/%s: Disabling Cors configuration", ingress.Namespace, ingress.Name)
		return
	}
	logger.Tracef("Ingress %s/%s: Enabling Cors configuration", ingress.Namespace, ingress.Name)
	var acl string
	acl, err = c.handleResponseCorsOrigin(ingress, result)
	if err != nil {
		logger.Error(err)
		return
	}
	c.handleResponseCorsMethod(ingress, acl, result)
	c.handleResponseCorsCredential(ingress, acl, result)
	c.handleResponseCorsHeaders(ingress, acl, result)
	c.handleResponseCorsMaxAge(ingress, acl, result)
}

func (c *HAProxyController) handleResponseCorsOrigin(ingress *store.Ingress, result *haproxy.Rules) (acl string, err error) {
	annOrigin := annotations.GetValue("cors-allow-origin", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annOrigin == "" {
		return acl, fmt.Errorf("cors-allow-origin not defined")
	}
	logger.Trace("Cors acl processing")
	logger.Tracef("Ingress %s/%s: Configuring cors-allow-origin", ingress.Namespace, ingress.Name)

	// SetVar rule to capture Origin header
	originVar := fmt.Sprintf("origin.%s", utils.Hash([]byte(annOrigin)))
	reqSetVar := rules.ReqSetVar{
		Name:       originVar,
		Scope:      "txn",
		Expression: "req.hdr(origin)",
	}
	// SetHdr rule to set Access-Control-Allow-Origin
	// Access-Control-Allow-Origin = *
	acl = fmt.Sprintf("{ var(txn.%s) -m found }", originVar)
	resSetHdr := rules.SetHdr{
		HdrName:   "Access-Control-Allow-Origin",
		HdrFormat: "*",
		Response:  true,
		CondTest:  acl,
	}
	// Access-Control-Allow-Origin = <origin>
	if annOrigin != "*" {
		acl = fmt.Sprintf("{ var(txn.%s) -m reg %s }", originVar, annOrigin)
		resSetHdr.HdrFormat = "%[var(txn." + originVar + ")]"
		resSetHdr.CondTest = acl
	}
	result.Add(resSetHdr)
	result.Add(reqSetVar)
	return acl, nil
}

func (c *HAProxyController) handleResponseCorsMethod(ingress *store.Ingress, acl string, result *haproxy.Rules) {
	annotation := annotations.GetValue("cors-allow-methods", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annotation == "" {
		return
	}
	logger.Tracef("Ingress %s/%s: Configuring cors-allow-methods", ingress.Namespace, ingress.Name)
	existingHTTPMethods := map[string]struct{}{"GET": {}, "POST": {}, "PUT": {}, "DELETE": {}, "HEAD": {}, "CONNECT": {}, "OPTIONS": {}, "TRACE": {}, "PATCH": {}}
	value := annotation
	if value != "*" {
		value = strings.Join(strings.Fields(value), "") // strip spaces
		methods := strings.Split(value, ",")
		for i, method := range methods {
			methods[i] = strings.ToUpper(method)
			if _, ok := existingHTTPMethods[methods[i]]; !ok {
				logger.Errorf("Ingress %s/%s: Incorrect HTTP method '%s' in cors-allow-methods configuration", ingress.Namespace, ingress.Name, methods[i])
				continue
			}
		}
		value = "\"" + strings.Join(methods, ", ") + "\""
	}
	resSetHdr := rules.SetHdr{
		HdrName:   "Access-Control-Allow-Methods",
		HdrFormat: value,
		Response:  true,
		CondTest:  acl,
	}
	result.Add(resSetHdr)
}

func (c *HAProxyController) handleResponseCorsCredential(ingress *store.Ingress, acl string, result *haproxy.Rules) {
	annotation := annotations.GetValue("cors-allow-credentials", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annotation == "" {
		return
	}
	enabled, err := utils.GetBoolValue(annotation, "cors-allow-credentials")
	if err != nil {
		logger.Error(err)
		return
	}
	if !enabled {
		logger.Tracef("Ingress %s/%s: Deleting cors-allow-credentials configuration", ingress.Namespace, ingress.Name)
		return
	}
	logger.Tracef("Ingress %s/%s: Configuring cors-allow-credentials", ingress.Namespace, ingress.Name)
	resSetHdr := rules.SetHdr{
		HdrName:   "Access-Control-Allow-Credentials",
		HdrFormat: "\"true\"",
		Response:  true,
		CondTest:  acl,
	}
	result.Add(resSetHdr)
}

func (c *HAProxyController) handleResponseCorsHeaders(ingress *store.Ingress, acl string, result *haproxy.Rules) {
	annotation := annotations.GetValue("cors-allow-headers", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annotation == "" {
		return
	}
	logger.Tracef("Ingress %s/%s: Configuring cors-allow-headers", ingress.Namespace, ingress.Name)
	value := strings.Join(strings.Fields(annotation), "") // strip spaces
	resSetHdr := rules.SetHdr{
		HdrName:   "Access-Control-Allow-Headers",
		HdrFormat: "\"" + value + "\"",
		Response:  true,
		CondTest:  acl,
	}
	result.Add(resSetHdr)
}

func (c *HAProxyController) handleResponseCorsMaxAge(ingress *store.Ingress, acl string, result *haproxy.Rules) {
	logger.Trace("Cors max age processing")
	annotation := annotations.GetValue("cors-max-age", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	if annotation == "" {
		return
	}
	r, err := utils.ParseTime(annotation)
	if err != nil {
		logger.Error(err)
		return
	}
	maxage := *r / 1000
	if maxage < -1 {
		logger.Errorf("Ingress %s/%s: Invalid cors-max-age value %d", ingress.Namespace, ingress.Name, maxage)
		return
	}
	logger.Tracef("Ingress %s/%s: Configuring cors-max-age", ingress.Namespace, ingress.Name)
	resSetHdr := rules.SetHdr{
		HdrName:   "Access-Control-Max-Age",
		HdrFormat: fmt.Sprintf("\"%d\"", maxage),
		Response:  true,
		CondTest:  acl,
	}
	result.Add(resSetHdr)
}

func tlsEnabled(ingress *store.Ingress) bool {
	for _, tls := range ingress.TLS {
		if tls.Status != DELETED {
			return true
		}
	}
	return false
}
