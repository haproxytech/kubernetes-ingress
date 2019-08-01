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

	"github.com/haproxytech/models"
)

func (c *HAProxyController) RequestsHTTPRefresh(transaction *models.Transaction) (needsReload bool, err error) {
	needsReload = false
	if c.cfg.HTTPRequestsStatus == EMPTY {
		return needsReload, nil
	}
	nativeAPI := c.NativeAPI

	err = nil
	for err == nil {
		err = nativeAPI.Configuration.DeleteHTTPRequestRule(0, "frontend", FrontendHTTP, transaction.ID, 0)
	}
	err = nil
	for err == nil {
		err = nativeAPI.Configuration.DeleteHTTPRequestRule(0, "frontend", FrontendHTTPS, transaction.ID, 0)
	}
	//INFO: order is reversed, first you insert last ones
	if len(c.cfg.HTTPRequests[HTTP_REDIRECT]) > 0 {
		request1 := &c.cfg.HTTPRequests[HTTP_REDIRECT][0]

		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", FrontendHTTP, request1, transaction.ID, 0)
		LogErr(err)
	}
	if len(c.cfg.HTTPRequests[RATE_LIMIT]) > 0 {
		request1 := &c.cfg.HTTPRequests[RATE_LIMIT][0]
		request2 := &c.cfg.HTTPRequests[RATE_LIMIT][1]

		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", FrontendHTTP, request2, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", FrontendHTTP, request1, transaction.ID, 0)
		LogErr(err)

		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", FrontendHTTPS, request2, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", FrontendHTTPS, request1, transaction.ID, 0)
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
			err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", FrontendHTTP, &request, transaction.ID, 0)
			LogErr(err)
			err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", FrontendHTTPS, &request, transaction.ID, 0)
			LogErr(err)
		}
	}
	if c.cfg.HTTPRequestsStatus != EMPTY {
		needsReload = true
	}

	return needsReload, nil
}

func (c *HAProxyController) requestsTCPRefresh(transaction *models.Transaction) (needsReload bool, err error) {
	needsReload = false
	if c.cfg.TCPRequestsStatus == EMPTY {
		return needsReload, nil
	}
	nativeAPI := c.NativeAPI

	err = nil
	for err == nil {
		err = nativeAPI.Configuration.DeleteTCPRequestRule(0, "frontend", FrontendHTTP, transaction.ID, 0)
	}
	err = nil
	for err == nil {
		err = nativeAPI.Configuration.DeleteTCPRequestRule(0, "frontend", FrontendHTTPS, transaction.ID, 0)
	}

	if len(c.cfg.TCPRequests[RATE_LIMIT]) > 0 {
		request1 := &c.cfg.TCPRequests[RATE_LIMIT][0]
		request2 := &c.cfg.TCPRequests[RATE_LIMIT][1]

		err = nativeAPI.Configuration.CreateTCPRequestRule("frontend", FrontendHTTP, request1, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateTCPRequestRule("frontend", FrontendHTTP, request2, transaction.ID, 0)
		LogErr(err)

		err = nativeAPI.Configuration.CreateTCPRequestRule("frontend", FrontendHTTPS, request1, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateTCPRequestRule("frontend", FrontendHTTPS, request2, transaction.ID, 0)
		LogErr(err)
	}
	if c.cfg.TCPRequestsStatus != EMPTY {
		needsReload = true
	}
	return needsReload, nil
}
