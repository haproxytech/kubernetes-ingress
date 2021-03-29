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

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// Configuration represents k8s state

type Configuration struct {
	MapFiles       haproxy.Maps
	HAProxyRules   *haproxy.Rules
	Certificates   *haproxy.Certificates
	ActiveBackends map[string]struct{}
	HTTPS          bool
	SSLPassthrough bool
}

// Init initialize configuration
func (c *Configuration) Init() {
	c.MapFiles = haproxy.NewMapFiles(MapDir)
	c.MapFiles.SetPreserve(true, SNI, HOST, PATH_EXACT, PATH_PREFIX)
	logger.Panic(c.HAProxyRulesInit())
	c.Certificates = haproxy.NewCertificates(CaCertDir, FrontendCertDir, BackendCertDir)
	c.ActiveBackends = make(map[string]struct{})
}

// Clean cleans all the statuses of various data that was changed
// deletes them completely or just resets them if needed
func (c *Configuration) Clean() {
	c.MapFiles.Clean()
	c.Certificates.Clean()
	logger.Panic(c.HAProxyRulesInit())
	rateLimitTables = []string{}
}

func (c *Configuration) HAProxyRulesInit() error {
	if c.HAProxyRules == nil {
		c.HAProxyRules = haproxy.NewRules()
	} else {
		c.HAProxyRules.Clean(FrontendHTTP, FrontendHTTPS, FrontendSSL)
	}
	var errors utils.Errors
	errors.Add(
		// ForwardedProto rule
		c.HAProxyRules.AddRule(rules.SetHdr{
			ForwardedProto: true,
		}, "", FrontendHTTPS),
		// txn.base var used for logging
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "base",
			Scope:      "txn",
			Expression: "base",
		}, "", FrontendHTTP, FrontendHTTPS),
		// Backend switching rules.
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "path",
			Scope:      "txn",
			Expression: "path",
		}, "", FrontendHTTP, FrontendHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "host",
			Scope:      "txn",
			Expression: fmt.Sprintf("req.hdr(Host),field(1,:),lower,map(%s)", haproxy.GetMapPath(HOST)),
		}, "", FrontendHTTP, FrontendHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "host",
			Scope:      "txn",
			Expression: fmt.Sprintf("req.hdr(Host),field(1,:),regsub(^[^.]*,,),lower,map(%s,'')", haproxy.GetMapPath(HOST)),
			CondTest:   "!{ var(txn.host) -m found }",
		}, "", FrontendHTTP, FrontendHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "match",
			Scope:      "txn",
			Expression: fmt.Sprintf("var(txn.host),concat(,txn.path,),map(%s)", haproxy.GetMapPath(PATH_EXACT)),
		}, "", FrontendHTTP, FrontendHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "match",
			Scope:      "txn",
			Expression: fmt.Sprintf("var(txn.host),concat(,txn.path,),map_beg(%s)", haproxy.GetMapPath(PATH_PREFIX)),
			CondTest:   "!{ var(txn.match) -m found }",
		}, "", FrontendHTTP, FrontendHTTPS),
	)

	return errors.Result()
}
