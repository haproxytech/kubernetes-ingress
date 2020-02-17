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
	"sort"
	"strconv"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models"
)

func (c *HAProxyController) RequestsHTTPRefresh() (needsReload bool, err error) {
	needsReload = false
	if c.cfg.HTTPRequestsStatus == EMPTY {
		return needsReload, nil
	}

	c.frontendHTTPRequestRuleDeleteAll(FrontendHTTP)
	c.frontendHTTPRequestRuleDeleteAll(FrontendHTTPS)
	//INFO: order is reversed, first you insert last ones
	if len(c.cfg.HTTPRequests[HTTP_REDIRECT]) > 0 {
		err = c.frontendHTTPRequestRuleCreate(FrontendHTTP, c.cfg.HTTPRequests[HTTP_REDIRECT][0])
		utils.LogErr(err)
	}
	if len(c.cfg.HTTPRequests[RATE_LIMIT]) > 0 {

		err = c.frontendHTTPRequestRuleCreate(FrontendHTTP, c.cfg.HTTPRequests[RATE_LIMIT][1])
		utils.LogErr(err)
		err = c.frontendHTTPRequestRuleCreate(FrontendHTTP, c.cfg.HTTPRequests[RATE_LIMIT][0])
		utils.LogErr(err)

		err = c.frontendHTTPRequestRuleCreate(FrontendHTTPS, c.cfg.HTTPRequests[RATE_LIMIT][1])
		utils.LogErr(err)
		err = c.frontendHTTPRequestRuleCreate(FrontendHTTPS, c.cfg.HTTPRequests[RATE_LIMIT][0])
		utils.LogErr(err)
	}
	xforwardedprotoRule := models.HTTPRequestRule{
		ID:        utils.PtrInt64(0),
		Type:      "set-header",
		HdrName:   "X-Forwarded-Proto",
		HdrFormat: "https",
		Cond:      "if",
		CondTest:  "{ ssl_fc }",
	}
	err = c.frontendHTTPRequestRuleCreate(FrontendHTTPS, xforwardedprotoRule)
	utils.LogErr(err)

	sortedList := []string{}
	exclude := map[string]struct{}{
		HTTP_REDIRECT: struct{}{},
		RATE_LIMIT:    struct{}{},
	}
	for name := range c.cfg.HTTPRequests {
		_, excluding := exclude[name]
		if !excluding {
			sortedList = append(sortedList, name)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(sortedList))) // reverse order
	for _, name := range sortedList {
		for _, request := range c.cfg.HTTPRequests[name] {
			err = c.frontendHTTPRequestRuleCreate(FrontendHTTP, request)
			utils.LogErr(err)
			err = c.frontendHTTPRequestRuleCreate(FrontendHTTPS, request)
			utils.LogErr(err)
		}
	}
	if c.cfg.HTTPRequestsStatus != EMPTY {
		needsReload = true
	}

	return needsReload, nil
}

func (c *HAProxyController) requestsTCPRefresh() (needsReload bool, err error) {
	needsReload = false
	if c.cfg.TCPRequestsStatus == EMPTY {
		return needsReload, nil
	}

	// Frontends HTTP and HTTPS
	c.frontendTCPRequestRuleDeleteAll(FrontendHTTP)
	c.frontendTCPRequestRuleDeleteAll(FrontendHTTPS)

	if len(c.cfg.TCPRequests[RATE_LIMIT]) > 0 {
		err = c.frontendTCPRequestRuleCreate(FrontendHTTP, c.cfg.TCPRequests[RATE_LIMIT][0])
		utils.LogErr(err)
		err = c.frontendTCPRequestRuleCreate(FrontendHTTP, c.cfg.TCPRequests[RATE_LIMIT][1])
		utils.LogErr(err)

		err = c.frontendTCPRequestRuleCreate(FrontendHTTPS, c.cfg.TCPRequests[RATE_LIMIT][0])
		utils.LogErr(err)
		err = c.frontendTCPRequestRuleCreate(FrontendHTTPS, c.cfg.TCPRequests[RATE_LIMIT][1])
		utils.LogErr(err)
	}

	if !c.cfg.SSLPassthrough {
		return true, nil
	}

	// SSL Frontend
	c.frontendTCPRequestRuleDeleteAll(FrontendSSL)

	// Fixed SSLpassthrough rules
	err = c.frontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
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

	sortedList := []string{}
	exclude := map[string]struct{}{
		RATE_LIMIT: struct{}{},
	}
	for name := range c.cfg.TCPRequests {
		_, excluding := exclude[name]
		if !excluding {
			sortedList = append(sortedList, name)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(sortedList))) // reverse order
	for _, name := range sortedList {
		for _, request := range c.cfg.TCPRequests[name] {
			err = c.frontendTCPRequestRuleCreate(FrontendSSL, request)
			utils.LogErr(err)
		}
	}

	return true, nil
}

func (c *HAProxyController) handleHTTPRedirect(usingHTTPS bool) (reloadRequested bool, err error) {
	//see if we need to add redirect to https redirect scheme https if !{ ssl_fc }
	// no need for error checking, we have default value,
	//if not defined as false, we always do redirect
	reloadRequested = false
	sslRedirect, _ := GetValueFromAnnotations("ssl-redirect", c.cfg.ConfigMap.Annotations)
	enabled, err := utils.GetBoolValue(sslRedirect.Value, "ssl-redirect")
	if err != nil {
		return false, err
	}
	if !usingHTTPS {
		enabled = false
	}
	var state Status
	if enabled {
		if !c.cfg.SSLRedirect {
			c.cfg.SSLRedirect = true
			state = MODIFIED
		}
	} else if c.cfg.SSLRedirect {
		c.cfg.SSLRedirect = false
		state = DELETED
	}
	redirectCode := int64(302)
	annRedirectCode, _ := GetValueFromAnnotations("ssl-redirect-code", c.cfg.ConfigMap.Annotations)
	if value, err := strconv.ParseInt(annRedirectCode.Value, 10, 64); err == nil {
		redirectCode = value
	}
	if annRedirectCode.Status != "" {
		state = MODIFIED
	}
	rule := models.HTTPRequestRule{
		ID:         utils.PtrInt64(0),
		Type:       "redirect",
		RedirCode:  redirectCode,
		RedirValue: "https",
		RedirType:  "scheme",
		Cond:       "if",
		CondTest:   "!{ ssl_fc }",
	}
	switch state {
	case MODIFIED:
		c.cfg.HTTPRequests[HTTP_REDIRECT] = []models.HTTPRequestRule{rule}
		c.cfg.HTTPRequestsStatus = MODIFIED
		reloadRequested = true
	case DELETED:
		c.cfg.HTTPRequests[HTTP_REDIRECT] = []models.HTTPRequestRule{}
		c.cfg.HTTPRequestsStatus = MODIFIED
		reloadRequested = true
	}
	return reloadRequested, nil
}
