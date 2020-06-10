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

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

type FrontendHTTPReqs map[uint64]models.HTTPRequestRule
type FrontendHTTPRsps map[uint64]models.HTTPResponseRule
type FrontendTCPReqs map[uint64]models.TCPRequestRule
type BackendHTTPReqs struct {
	modified bool
	rules    map[Rule]models.HTTPRequestRule
}

type Rule string

type rateLimitTable struct {
	size   *int64
	period *int64
}

const (
	//nolint
	BLACKLIST Rule = "blacklist"
	//nolint
	RATE_LIMIT Rule = "rate-limit"
	//nolint
	SET_HOST Rule = "set-host"
	//nolint
	SSL_REDIRECT Rule = "ssl-redirect"
	//nolint
	PATH_REWRITE Rule = "path-rewrite"
	//nolint
	PROXY_PROTOCOL Rule = "proxy-protocol"
	//nolint
	REQUEST_CAPTURE Rule = "request-capture"
	//nolint
	REQUEST_SET_HEADER Rule = "request-set-header"
	//nolint
	RESPONSE_SET_HEADER Rule = "response-set-header"
	//nolint
	WHITELIST Rule = "whitelist"
)

func (c *HAProxyController) FrontendHTTPRspsRefresh() (reload bool) {
	if c.cfg.FrontendRulesStatus[HTTP] == EMPTY {
		return false
	}

	// DELETE RULES
	c.Client.FrontendHTTPResponseRuleDeleteAll(FrontendHTTP)
	c.Client.FrontendHTTPResponseRuleDeleteAll(FrontendHTTPS)

	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS} {
		// RESPONSE_SET_HEADER
		for key, httpRule := range c.cfg.FrontendHTTPRspRules[RESPONSE_SET_HEADER] {
			c.cfg.MapFiles.Modified(key)
			c.Logger.Error(c.Client.FrontendHTTPResponseRuleCreate(frontend, httpRule))
		}
	}
	return true
}

func (c *HAProxyController) FrontendHTTPReqsRefresh() (reload bool) {
	if c.cfg.FrontendRulesStatus[HTTP] == EMPTY {
		return false
	}

	c.Logger.Debug("Updating HTTP request rules for HTTP and HTTPS frontends")
	// DELETE RULES
	c.Client.FrontendHTTPRequestRuleDeleteAll(FrontendHTTP)
	c.Client.FrontendHTTPRequestRuleDeleteAll(FrontendHTTPS)
	//STATIC: FORWARDED_PRTOTO
	xforwardedprotoRule := models.HTTPRequestRule{
		Index:     utils.PtrInt64(0),
		Type:      "set-header",
		HdrName:   "X-Forwarded-Proto",
		HdrFormat: "https",
		Cond:      "if",
		CondTest:  "{ ssl_fc }",
	}
	c.Logger.Error(c.Client.FrontendHTTPRequestRuleCreate(FrontendHTTPS, xforwardedprotoRule))
	// SSL_REDIRECT
	for key, httpRule := range c.cfg.FrontendHTTPReqRules[SSL_REDIRECT] {
		c.cfg.MapFiles.Modified(key)
		c.Logger.Error(c.Client.FrontendHTTPRequestRuleCreate(FrontendHTTP, httpRule))
	}
	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS} {
		// REQUEST_SET_HEADER
		for key, httpRule := range c.cfg.FrontendHTTPReqRules[REQUEST_SET_HEADER] {
			c.cfg.MapFiles.Modified(key)
			c.Logger.Error(c.Client.FrontendHTTPRequestRuleCreate(frontend, httpRule))
		}
		// REQUEST_CAPTURE
		for key, httpRule := range c.cfg.FrontendHTTPReqRules[REQUEST_CAPTURE] {
			c.cfg.MapFiles.Modified(key)
			c.Logger.Error(c.Client.FrontendHTTPRequestRuleCreate(frontend, httpRule))
		}
		// STATIC: SET_VARIABLE txn.Base (for logging purpose)
		setVarBaseRule := models.HTTPRequestRule{
			Index:    utils.PtrInt64(0),
			Type:     "set-var",
			VarName:  "base",
			VarScope: "txn",
			VarExpr:  "base",
		}
		c.Logger.Error(c.Client.FrontendHTTPRequestRuleCreate(frontend, setVarBaseRule))
		// STATIC: SET_VARIABLE txn.Host (to use in http-response acls)
		setVarBaseRule = models.HTTPRequestRule{
			Index:    utils.PtrInt64(0),
			Type:     "set-var",
			VarName:  "Host",
			VarScope: "txn",
			VarExpr:  "req.hdr(Host)",
		}
		c.Logger.Error(c.Client.FrontendHTTPRequestRuleCreate(frontend, setVarBaseRule))
		// RATE_LIMIT
		for tableName, table := range rateLimitTables {
			_, err := c.Client.BackendGet(tableName)
			if err != nil {
				err := c.Client.BackendCreate(models.Backend{
					Name: tableName,
					StickTable: &models.BackendStickTable{
						Type:  "ip",
						Size:  table.size,
						Store: fmt.Sprintf("http_req_rate(%d)", *table.period),
					},
				})
				c.Logger.Error(err)
			}
		}
		for key, httpRule := range c.cfg.FrontendHTTPReqRules[RATE_LIMIT] {
			c.cfg.MapFiles.Modified(key)
			c.Logger.Error(c.Client.FrontendHTTPRequestRuleCreate(frontend, httpRule))
		}
		// BLACKLIST
		for _, httpRule := range c.cfg.FrontendHTTPReqRules[BLACKLIST] {
			c.Logger.Error(c.Client.FrontendHTTPRequestRuleCreate(frontend, httpRule))
		}
		// WHITELIST
		for _, httpRule := range c.cfg.FrontendHTTPReqRules[WHITELIST] {
			c.Logger.Error(c.Client.FrontendHTTPRequestRuleCreate(frontend, httpRule))
		}
	}
	return true
}

func (c *HAProxyController) FrontendTCPreqsRefresh() (reload bool) {
	if c.cfg.FrontendRulesStatus[TCP] == EMPTY {
		return false
	}
	c.Logger.Debug("Updating TCP request rules for HTTP and HTTPS frontends")
	// HTTP and HTTPS Frrontends
	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS} {
		// DELETE RULES
		c.Client.FrontendTCPRequestRuleDeleteAll(frontend)
		// PROXY_PROTCOL
		if len(c.cfg.FrontendTCPRules[PROXY_PROTOCOL]) > 0 {
			c.Logger.Error(c.Client.FrontendTCPRequestRuleCreate(frontend, c.cfg.FrontendTCPRules[PROXY_PROTOCOL][0]))
		}
	}
	if !c.cfg.SSLPassthrough {
		return true
	}

	// SSL Frontend for SSL_PASSTHROUGH
	c.Client.FrontendTCPRequestRuleDeleteAll(FrontendSSL)
	// STATIC: Accept content
	err := c.Client.FrontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Action:   "accept",
		Type:     "content",
		Cond:     "if",
		CondTest: "{ req_ssl_hello_type 1 }",
	})
	c.Logger.Error(err)
	// REQUEST_CAPTURE
	for key, tcpRule := range c.cfg.FrontendTCPRules[REQUEST_CAPTURE] {
		c.cfg.MapFiles.Modified(key)
		c.Logger.Error(c.Client.FrontendTCPRequestRuleCreate(FrontendSSL, tcpRule))
	}
	// STATIC: Set-var rule used to log SNI
	err = c.Client.FrontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Action:   "set-var",
		VarName:  "sni",
		VarScope: "sess",
		Expr:     "req_ssl_sni",
		Type:     "content",
	})
	c.Logger.Error(err)
	// STATIC: Inspect delay
	err = c.Client.FrontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Index:   utils.PtrInt64(0),
		Type:    "inspect-delay",
		Timeout: utils.PtrInt64(5000),
	})
	c.Logger.Error(err)
	// BLACKLIST
	for key, tcpRule := range c.cfg.FrontendTCPRules[BLACKLIST] {
		c.cfg.MapFiles.Modified(key)
		c.Logger.Error(c.Client.FrontendTCPRequestRuleCreate(FrontendSSL, tcpRule))
	}
	// WHITELIST
	for key, tcpRule := range c.cfg.FrontendTCPRules[WHITELIST] {
		c.cfg.MapFiles.Modified(key)
		c.Logger.Error(c.Client.FrontendTCPRequestRuleCreate(FrontendSSL, tcpRule))
	}
	// PROXY_PROTCOL
	if len(c.cfg.FrontendTCPRules[PROXY_PROTOCOL]) > 0 {
		c.Logger.Error(c.Client.FrontendTCPRequestRuleCreate(FrontendSSL, c.cfg.FrontendTCPRules[PROXY_PROTOCOL][0]))
	}
	return true
}

func (c *HAProxyController) BackendHTTPReqsRefresh() (reload bool) {
	for backendName, httpReqs := range c.cfg.BackendHTTPRules {
		if httpReqs.modified {
			c.Logger.Debugf("Updating HTTP request rules for backend '%s'", backendName)
			reload = true
			c.Client.BackendHTTPRequestRuleDeleteAll(backendName)
			if len(httpReqs.rules) == 0 {
				delete(c.cfg.BackendHTTPRules, backendName)
			} else {
				for _, httpRule := range httpReqs.rules {
					c.Logger.Error(c.Client.BackendHTTPRequestRuleCreate(backendName, httpRule))
				}
			}
			httpReqs.modified = false
			c.cfg.BackendHTTPRules[backendName] = httpReqs
		}
	}
	return reload
}
