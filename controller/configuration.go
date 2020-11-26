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
	"github.com/haproxytech/kubernetes-ingress/controller/ingress"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

//Configuration represents k8s state

type Configuration struct {
	MapFiles       haproxy.Maps
	HAProxyRules   haproxy.Rules
	IngressRoutes  ingress.Routes
	HTTPS          bool
	SSLPassthrough bool
	UsedCerts      map[string]struct{}
}

//Init initialize configuration
func (c *Configuration) Init(mapDir string) {

	c.MapFiles = haproxy.NewMapFiles(mapDir)
	c.IngressRoutes = ingress.Routes{}
	logger.Panic(c.HAProxyRulesInit())
}

//Clean cleans all the statuses of various data that was changed
//deletes them completely or just resets them if needed
func (c *Configuration) Clean() {
	c.MapFiles.Clean()
	c.IngressRoutes = ingress.Routes{}
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
			Expression: "path",
		}, FrontendHTTP, FrontendHTTPS),
	)
	c.MapFiles.AppendRow(0, "# Ingress SNIs")
	c.MapFiles.AppendRow(1, "# Ingress Hosts")
	c.MapFiles.AppendRow(2, "# Ingress exact paths ")
	c.MapFiles.AppendRow(3, "# Ingress prefix paths ")
	errors.Add(
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "host",
			Scope:      "txn",
			Expression: fmt.Sprintf("req.hdr(Host),field(1,:),lower,map_end(%s,'')", haproxy.MapID(1).Path()),
		}, FrontendHTTP, FrontendHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "match",
			Scope:      "txn",
			Expression: fmt.Sprintf("var(txn.host),concat(,txn.path,),map(%s)", haproxy.MapID(2).Path()),
		}, FrontendHTTP, FrontendHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "match",
			Scope:      "txn",
			Expression: fmt.Sprintf("var(txn.host),concat(,txn.path,),map_beg(%s)", haproxy.MapID(3).Path()),
			CondTest:   "!{ var(txn.match) -m found }",
		}, FrontendHTTP, FrontendHTTPS),
	)

	return errors.Result()
}
