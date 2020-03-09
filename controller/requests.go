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
	RATE_LIMIT Rule = "rate-limit"
	//nolint
	HTTP_REDIRECT Rule = "http-redirect"
	//nolint
	REQUEST_CAPTURE Rule = "request-capture"
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
	// FORWARDED_PRTOTO
	xforwardedprotoRule := models.HTTPRequestRule{
		ID:        utils.PtrInt64(0),
		Type:      "set-header",
		HdrName:   "X-Forwarded-Proto",
		HdrFormat: "https",
		Cond:      "if",
		CondTest:  "{ ssl_fc }",
	}
	utils.LogErr(c.frontendHTTPRequestRuleCreate(FrontendHTTPS, xforwardedprotoRule))
	// HTTP_REDIRECT
	for _, httpRule := range c.cfg.HTTPRequests[REQUEST_CAPTURE] {
		utils.LogErr(c.frontendHTTPRequestRuleCreate(FrontendHTTP, httpRule))
	}
	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS} {
		// REQUEST_CAPTURE
		for _, httpRule := range c.cfg.HTTPRequests[REQUEST_CAPTURE] {
			utils.LogErr(c.frontendHTTPRequestRuleCreate(frontend, httpRule))
		}
		// RATE_LIMIT
		if len(c.cfg.HTTPRequests[RATE_LIMIT]) > 0 {
			utils.LogErr(c.frontendHTTPRequestRuleCreate(frontend, c.cfg.HTTPRequests[RATE_LIMIT][1]))
			utils.LogErr(c.frontendHTTPRequestRuleCreate(frontend, c.cfg.HTTPRequests[RATE_LIMIT][0]))
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
	}
	if !c.cfg.SSLPassthrough {
		return true
	}

	// SSL Frontend for SSL_PASSTHROUGH
	c.frontendTCPRequestRuleDeleteAll(FrontendSSL)
	// REQUEST_CAPTURE
	for _, tcpRule := range c.cfg.TCPRequests[REQUEST_CAPTURE] {
		utils.LogErr(c.frontendTCPRequestRuleCreate(FrontendSSL, tcpRule))
	}
	// RATE_LIMIT
	if len(c.cfg.TCPRequests[RATE_LIMIT]) > 0 {
		utils.LogErr(c.frontendTCPRequestRuleCreate(FrontendSSL, c.cfg.TCPRequests[RATE_LIMIT][1]))
		utils.LogErr(c.frontendTCPRequestRuleCreate(FrontendSSL, c.cfg.TCPRequests[RATE_LIMIT][0]))
	}
	// WHITELIST
	for _, tcpRule := range c.cfg.TCPRequests[WHITELIST] {
		utils.LogErr(c.frontendTCPRequestRuleCreate(FrontendSSL, tcpRule))
	}

	// Fixed SSLpassthrough rules
	err := c.frontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		ID:       utils.PtrInt64(0),
		Action:   "accept",
		Type:     "content",
		Cond:     "if",
		CondTest: "{ req_ssl_hello_type 1 }",
	})
	utils.LogErr(err)

	err = c.frontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		ID:      utils.PtrInt64(0),
		Type:    "inspect-delay",
		Timeout: utils.PtrInt64(5000),
	})
	utils.LogErr(err)

	return true
}
