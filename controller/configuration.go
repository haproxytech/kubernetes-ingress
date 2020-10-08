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
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/models/v2"
)

//Configuration represents k8s state

type Configuration struct {
	MapFiles                 haproxy.Maps
	FrontendHTTPReqRules     map[Rule]FrontendHTTPReqs
	FrontendHTTPRspRules     map[Rule]FrontendHTTPRsps
	FrontendTCPRules         map[Rule]FrontendTCPReqs
	FrontendRulesModified    map[Mode]bool
	BackendSwitchingRules    map[string]UseBackendRules
	BackendSwitchingModified map[string]struct{}
	BackendHTTPRules         map[string]BackendHTTPReqs
	HTTPS                    bool
	SSLPassthrough           bool
	UsedCerts                map[string]struct{}
}

//Init initialize configuration
func (c *Configuration) Init(mapDir string, httpsEnabled bool) {

	c.FrontendHTTPReqRules = make(map[Rule]FrontendHTTPReqs)
	for _, rule := range []Rule{BLACKLIST, SSL_REDIRECT, RATE_LIMIT, REQUEST_CAPTURE, REQUEST_SET_HEADER, WHITELIST} {
		c.FrontendHTTPReqRules[rule] = make(map[uint64]models.HTTPRequestRule)
	}
	c.FrontendHTTPRspRules = make(map[Rule]FrontendHTTPRsps)
	for _, rule := range []Rule{RESPONSE_SET_HEADER} {
		c.FrontendHTTPRspRules[rule] = make(map[uint64]models.HTTPResponseRule)
	}
	c.FrontendTCPRules = make(map[Rule]FrontendTCPReqs)
	for _, rule := range []Rule{BLACKLIST, REQUEST_CAPTURE, PROXY_PROTOCOL, WHITELIST} {
		c.FrontendTCPRules[rule] = make(map[uint64]models.TCPRequestRule)
	}
	c.FrontendRulesModified = map[Mode]bool{
		HTTP: false,
		TCP:  false,
	}
	c.MapFiles = haproxy.NewMapFiles(mapDir)

	sslRedirectEnabled = make(map[string]struct{})
	rateLimitTables = make(map[string]rateLimitTable)

	c.BackendSwitchingRules = make(map[string]UseBackendRules)
	c.BackendSwitchingModified = make(map[string]struct{})
	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS, FrontendSSL} {
		c.BackendSwitchingRules[frontend] = UseBackendRules{}
	}
	c.BackendHTTPRules = make(map[string]BackendHTTPReqs)

	c.HTTPS = httpsEnabled
}

//Clean cleans all the statuses of various data that was changed
//deletes them completely or just resets them if needed
func (c *Configuration) Clean() {
	c.MapFiles.Clean()
	for rule := range c.FrontendHTTPReqRules {
		c.FrontendHTTPReqRules[rule] = make(map[uint64]models.HTTPRequestRule)
	}
	for rule := range c.FrontendHTTPRspRules {
		c.FrontendHTTPRspRules[rule] = make(map[uint64]models.HTTPResponseRule)
	}
	for rule := range c.FrontendTCPRules {
		c.FrontendTCPRules[rule] = make(map[uint64]models.TCPRequestRule)
	}
	c.FrontendRulesModified[HTTP] = false
	c.FrontendRulesModified[TCP] = false
}
