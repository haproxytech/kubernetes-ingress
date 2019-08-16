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

package main

import (
	"sort"
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
		LogErr(err)
	}
	if len(c.cfg.HTTPRequests[RATE_LIMIT]) > 0 {

		err = c.frontendHTTPRequestRuleCreate(FrontendHTTP, c.cfg.HTTPRequests[RATE_LIMIT][1])
		LogErr(err)
		err = c.frontendHTTPRequestRuleCreate(FrontendHTTP, c.cfg.HTTPRequests[RATE_LIMIT][0])
		LogErr(err)

		err = c.frontendHTTPRequestRuleCreate(FrontendHTTPS, c.cfg.HTTPRequests[RATE_LIMIT][1])
		LogErr(err)
		err = c.frontendHTTPRequestRuleCreate(FrontendHTTPS, c.cfg.HTTPRequests[RATE_LIMIT][0])
		LogErr(err)
	}

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
			LogErr(err)
			err = c.frontendHTTPRequestRuleCreate(FrontendHTTPS, request)
			LogErr(err)
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

	c.frontendTCPRequestRuleDeleteAll(FrontendHTTP)
	c.frontendTCPRequestRuleDeleteAll(FrontendHTTPS)

	if len(c.cfg.TCPRequests[RATE_LIMIT]) > 0 {
		err = c.frontendTCPRequestRuleCreate(FrontendHTTP, c.cfg.TCPRequests[RATE_LIMIT][0])
		LogErr(err)
		err = c.frontendTCPRequestRuleCreate(FrontendHTTP, c.cfg.TCPRequests[RATE_LIMIT][1])
		LogErr(err)

		err = c.frontendTCPRequestRuleCreate(FrontendHTTPS, c.cfg.TCPRequests[RATE_LIMIT][0])
		LogErr(err)
		err = c.frontendTCPRequestRuleCreate(FrontendHTTPS, c.cfg.TCPRequests[RATE_LIMIT][1])
		LogErr(err)
	}
	if c.cfg.TCPRequestsStatus != EMPTY {
		needsReload = true
	}
	return needsReload, nil
}
