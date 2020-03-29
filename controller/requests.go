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
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models"
)

type HTTPRequestRules map[uint64]models.HTTPRequestRule
type TCPRequestRules map[uint64]models.TCPRequestRule

type Rule string

const (
	//nolint
	BLACKLIST Rule = "blacklist"
	//nolint
	RATE_LIMIT Rule = "rate-limit"
	//nolint
	SSL_REDIRECT Rule = "ssl-redirect"
	//nolint
	PROXY_PROTOCOL Rule = "proxy-protocol"
	//nolint
	REQUEST_CAPTURE Rule = "request-capture"
	//nolint
	REQUEST_SET_HEADER Rule = "request-set-header"
	//nolint
	WHITELIST Rule = "whitelist"
)

func (c *HAProxyController) RequestsHTTPRefresh() (needsReload bool) {
	if c.cfg.HTTPRequestsStatus == EMPTY {
		return false
	}

	// DELETE RULES
	c.frontendHTTPRequestRuleDeleteAll(FrontendHTTP)
	c.frontendHTTPRequestRuleDeleteAll(FrontendHTTPS)
	//STATIC: FORWARDED_PRTOTO
	xforwardedprotoRule := models.HTTPRequestRule{
		Index:     utils.PtrInt64(0),
		Type:      "set-header",
		HdrName:   "X-Forwarded-Proto",
		HdrFormat: "https",
		Cond:      "if",
		CondTest:  "{ ssl_fc }",
	}
	utils.LogErr(c.frontendHTTPRequestRuleCreate(FrontendHTTPS, xforwardedprotoRule))
	// SSL_REDIRECT
	for key, httpRule := range c.cfg.HTTPRequests[SSL_REDIRECT] {
		c.cfg.MapFiles.Modified(key)
		utils.LogErr(c.frontendHTTPRequestRuleCreate(FrontendHTTP, httpRule))
	}
	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS} {
		// REQUEST_SET_HEADER
		for key, httpRule := range c.cfg.HTTPRequests[REQUEST_SET_HEADER] {
			c.cfg.MapFiles.Modified(key)
			utils.LogErr(c.frontendHTTPRequestRuleCreate(frontend, httpRule))
		}
		// REQUEST_CAPTURE
		for key, httpRule := range c.cfg.HTTPRequests[REQUEST_CAPTURE] {
			c.cfg.MapFiles.Modified(key)
			utils.LogErr(c.frontendHTTPRequestRuleCreate(frontend, httpRule))
		}
		// STATIC: SET_VARIABLE txn.Base (for logging purpose)
		setVarBaseRule := models.HTTPRequestRule{
			Index:    utils.PtrInt64(0),
			Type:     "set-var",
			VarName:  "base",
			VarScope: "txn",
			VarExpr:  "base",
		}
		utils.LogErr(c.frontendHTTPRequestRuleCreate(frontend, setVarBaseRule))
		// RATE_LIMIT
		if len(c.cfg.HTTPRequests[RATE_LIMIT]) > 0 {
			utils.LogErr(c.frontendHTTPRequestRuleCreate(frontend, c.cfg.HTTPRequests[RATE_LIMIT][1]))
			utils.LogErr(c.frontendHTTPRequestRuleCreate(frontend, c.cfg.HTTPRequests[RATE_LIMIT][0]))
		}
		// BLACKLIST
		for _, httpRule := range c.cfg.HTTPRequests[BLACKLIST] {
			utils.LogErr(c.frontendHTTPRequestRuleCreate(frontend, httpRule))
		}
		// WHITELIST
		for _, httpRule := range c.cfg.HTTPRequests[WHITELIST] {
			utils.LogErr(c.frontendHTTPRequestRuleCreate(frontend, httpRule))
		}
	}
	return true
}

func (c *HAProxyController) RequestsTCPRefresh() (needsReload bool) {
	if c.cfg.TCPRequestsStatus == EMPTY {
		return false
	}

	// HTTP and HTTPS Frrontends
	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS} {
		// DELETE RULES
		c.frontendTCPRequestRuleDeleteAll(frontend)
		// RATE_LIMIT
		if len(c.cfg.HTTPRequests[RATE_LIMIT]) > 0 {
			utils.LogErr(c.frontendTCPRequestRuleCreate(frontend, c.cfg.TCPRequests[RATE_LIMIT][0]))
			utils.LogErr(c.frontendTCPRequestRuleCreate(frontend, c.cfg.TCPRequests[RATE_LIMIT][1]))
		}
		// PROXY_PROTCOL
		if len(c.cfg.TCPRequests[PROXY_PROTOCOL]) > 0 {
			utils.LogErr(c.frontendTCPRequestRuleCreate(frontend, c.cfg.TCPRequests[PROXY_PROTOCOL][0]))
		}
	}
	if !c.cfg.SSLPassthrough {
		return true
	}

	// SSL Frontend for SSL_PASSTHROUGH
	c.frontendTCPRequestRuleDeleteAll(FrontendSSL)
	// STATIC: Accept content
	err := c.frontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Action:   "accept",
		Type:     "content",
		Cond:     "if",
		CondTest: "{ req_ssl_hello_type 1 }",
	})
	utils.LogErr(err)
	// REQUEST_CAPTURE
	for key, tcpRule := range c.cfg.TCPRequests[REQUEST_CAPTURE] {
		c.cfg.MapFiles.Modified(key)
		utils.LogErr(c.frontendTCPRequestRuleCreate(FrontendSSL, tcpRule))
	}
	// RATE_LIMIT
	if len(c.cfg.TCPRequests[RATE_LIMIT]) > 0 {
		utils.LogErr(c.frontendTCPRequestRuleCreate(FrontendSSL, c.cfg.TCPRequests[RATE_LIMIT][1]))
		utils.LogErr(c.frontendTCPRequestRuleCreate(FrontendSSL, c.cfg.TCPRequests[RATE_LIMIT][0]))
	}
	// STATIC: Set-var rule used to log SNI
	err = c.frontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Index:  utils.PtrInt64(0),
		Action: "set-var(sess.sni) req_ssl_sni",
		Type:   "content",
	})
	utils.LogErr(err)
	// STATIC: Inspect delay
	err = c.frontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Index:   utils.PtrInt64(0),
		Type:    "inspect-delay",
		Timeout: utils.PtrInt64(5000),
	})
	utils.LogErr(err)
	// BLACKLIST
	for key, tcpRule := range c.cfg.TCPRequests[BLACKLIST] {
		c.cfg.MapFiles.Modified(key)
		utils.LogErr(c.frontendTCPRequestRuleCreate(FrontendSSL, tcpRule))
	}
	// WHITELIST
	for key, tcpRule := range c.cfg.TCPRequests[WHITELIST] {
		c.cfg.MapFiles.Modified(key)
		utils.LogErr(c.frontendTCPRequestRuleCreate(FrontendSSL, tcpRule))
	}
	// PROXY_PROTCOL
	if len(c.cfg.TCPRequests[PROXY_PROTOCOL]) > 0 {
		utils.LogErr(c.frontendTCPRequestRuleCreate(FrontendSSL, c.cfg.TCPRequests[PROXY_PROTOCOL][0]))
	}
	return true
}
