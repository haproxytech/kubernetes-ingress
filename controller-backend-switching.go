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
	"fmt"
	"log"
	"sort"

	"github.com/haproxytech/models"
)

type BackendSwitching struct {
	Modified bool
	Rules    map[string]BackendSwitchingRule
}
type BackendSwitchingRule struct {
	Host      string
	Path      string
	Backend   string
	Namespace string
}

func (c *HAProxyController) refreshBackendSwitching() (needsReload bool) {

	// Refresh use_backend rules
	activeBackends := make(map[string]struct{})
	needsReload = c.refreshHTTPBackendSwitching(activeBackends)
	needsReload = c.refreshTCPBackendSwitching(activeBackends) || needsReload
	if !needsReload {
		return false
	}

	// Add default and RateLimit backend
	frontend, _ := c.frontendGet(FrontendHTTP)
	activeBackends[frontend.DefaultBackend] = struct{}{}
	activeBackends["RateLimit"] = struct{}{}

	// Add Backends used by TCP services
	for tcpBackend := range c.cfg.TCPBackends {
		activeBackends[tcpBackend] = struct{}{}
	}

	// Delete unused backends
	allBackends, _ := c.backendsGet()
	for _, backend := range allBackends {
		_, ok := activeBackends[backend.Name]
		if !ok {
			err := c.backendDelete(backend.Name)
			LogErr(err)
		}
	}

	return true
}

func (c *HAProxyController) refreshHTTPBackendSwitching(activeBackends map[string]struct{}) (needsReload bool) {
	frontends := []string{FrontendHTTP, FrontendHTTPS}

	useBackendRules := c.cfg.UseBackendRules[ModeHTTP].Rules
	if !c.cfg.UseBackendRules[ModeHTTP].Modified {
		for _, rule := range useBackendRules {
			activeBackends[rule.Backend] = struct{}{}
		}
		return false
	}
	sortedList := []string{}
	for name := range useBackendRules {
		sortedList = append(sortedList, name)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(sortedList))) // reverse order

	for _, frontend := range frontends {
		var err error
		c.backendSwitchingRuleDeleteAll(frontend)
		for _, name := range sortedList {
			rule := useBackendRules[name]
			var condTest string
			if rule.Host != "" {
				condTest = fmt.Sprintf("{ req.hdr(host) -i %s } ", rule.Host)
			}
			if rule.Path != "" {
				condTest = fmt.Sprintf("%s{ path_beg %s }", condTest, rule.Path)
			}
			if condTest == "" {
				log.Println(fmt.Sprintf("Both Host and Path are empty for frontend %s with backend %s, SKIP", frontend, rule.Backend))
				continue
			}
			activeBackends[rule.Backend] = struct{}{}
			err = c.backendSwitchingRuleCreate(frontend, models.BackendSwitchingRule{
				Cond:     "if",
				CondTest: condTest,
				Name:     rule.Backend,
				ID:       ptrInt64(0),
			})
			LogErr(err)
		}
	}
	c.cfg.UseBackendRules[ModeHTTP].Modified = false
	return true
}

//  Refresh use_backend rules of the SSL Frontend
func (c *HAProxyController) refreshTCPBackendSwitching(activeBackends map[string]struct{}) (needsReload bool) {
	_, err := c.frontendGet(FrontendSSL)
	if err != nil {
		LogErr(err)
		return false
	}

	useBackendRules := c.cfg.UseBackendRules[ModeTCP].Rules
	if !c.cfg.UseBackendRules[ModeTCP].Modified {
		for _, rule := range useBackendRules {
			activeBackends[rule.Backend] = struct{}{}
		}
		return false
	}
	sortedList := []string{}
	for name := range useBackendRules {
		sortedList = append(sortedList, name)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(sortedList))) // reverse order

	c.backendSwitchingRuleDeleteAll(FrontendSSL)
	for _, name := range sortedList {
		rule := useBackendRules[name]
		if rule.Host == "" {
			log.Println(fmt.Sprintf("Empty SNI for backend %s, SKIP", rule.Backend))
			continue
		}
		activeBackends[rule.Backend] = struct{}{}
		err = c.backendSwitchingRuleCreate(FrontendSSL, models.BackendSwitchingRule{
			Cond:     "if",
			CondTest: fmt.Sprintf("{ req_ssl_sni -i %s } ", rule.Host),
			Name:     rule.Backend,
			ID:       ptrInt64(0),
		})
		LogErr(err)
	}
	c.cfg.UseBackendRules[ModeTCP].Modified = false
	return true
}
