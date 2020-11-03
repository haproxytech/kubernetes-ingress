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
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

//Configuration represents k8s state

type Configuration struct {
	MapFiles                 haproxy.Maps
	HAProxyRules             haproxy.Rules
	BackendSwitchingRules    map[string]UseBackendRules
	BackendSwitchingModified map[string]struct{}
	HTTPS                    bool
	SSLPassthrough           bool
	UsedCerts                map[string]struct{}
}

//Init initialize configuration
func (c *Configuration) Init(mapDir string) {

	c.MapFiles = haproxy.NewMapFiles(mapDir)
	logger.Panic(c.HAProxyRulesInit())

	c.BackendSwitchingRules = make(map[string]UseBackendRules)
	c.BackendSwitchingModified = make(map[string]struct{})
	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS, FrontendSSL} {
		c.BackendSwitchingRules[frontend] = UseBackendRules{}
	}
}

//Clean cleans all the statuses of various data that was changed
//deletes them completely or just resets them if needed
func (c *Configuration) Clean() {
	c.MapFiles.Clean()
	logger.Panic(c.HAProxyRulesInit())
	rateLimitTables = []string{}
}

func (c *Configuration) HAProxyRulesInit() error {
	c.HAProxyRules = haproxy.NewRules()
	var errors utils.Errors
	errors.Add(
		c.HAProxyRules.AddRule(rules.SetHdr{
			ForwardedProto: true,
		}, FrontendHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "base",
			Scope:      "txn",
			Expression: "base",
		}, FrontendHTTP, FrontendHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "path",
			Scope:      "txn",
			Expression: "path,lower",
		}, FrontendHTTP, FrontendHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "host",
			Scope:      "txn",
			Expression: "req.hdr(Host),field(1,:),lower",
		}, FrontendHTTP, FrontendHTTPS),
	)
	return errors.Result()
}
