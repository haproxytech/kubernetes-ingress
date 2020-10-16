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
	"sort"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type UseBackendRules map[string]UseBackendRule

type UseBackendRule struct {
	Host       string
	Path       string
	ExactMatch bool
	Backend    string
	Namespace  string
}

func (c *HAProxyController) addUseBackendRule(key string, rule UseBackendRule, frontends ...string) {
	for _, frontendName := range frontends {
		c.cfg.BackendSwitchingRules[frontendName][key] = rule
		c.cfg.BackendSwitchingModified[frontendName] = struct{}{}
	}
}

func (c *HAProxyController) deleteUseBackendRule(key string, frontends ...string) {
	for _, frontendName := range frontends {
		delete(c.cfg.BackendSwitchingRules[frontendName], key)
		c.cfg.BackendSwitchingModified[frontendName] = struct{}{}
	}
}

//  Recreate use_backend rules
func (c *HAProxyController) refreshBackendSwitching() (reload bool) {
	if len(c.cfg.BackendSwitchingModified) == 0 {
		return false
	}
	logger.Debug("Updating Backend Switching rules")
	frontends, err := c.Client.FrontendsGet()
	if err != nil {
		logger.Panic(err)
		return false
	}
	// Active backend will hold backends in use
	activeBackends := make(map[string]struct{})
	for rateLimitTable := range rateLimitTables {
		activeBackends[rateLimitTable] = struct{}{}
	}
	for _, frontend := range frontends {
		activeBackends[frontend.DefaultBackend] = struct{}{}
		useBackendRules, ok := c.cfg.BackendSwitchingRules[frontend.Name]
		if !ok {
			continue
		}
		sortedKeys := []string{}
		for key, rule := range useBackendRules {
			activeBackends[rule.Backend] = struct{}{}
			sortedKeys = append(sortedKeys, key)
		}
		if _, ok := c.cfg.BackendSwitchingModified[frontend.Name]; !ok {
			// No need to refresh rules if the use_backend rules
			// of the frontend were not updated
			continue
		}
		// host/path are part of use_backend keys, so sorting keys will
		// result in sorted use_backend rules where the longest path will match first.
		// Example:
		// use_backend service-abc if { var(txn.host) example } { var(txn.path) -m beg /a/b/c }
		// use_backend service-ab  if { var(txn.host) example } { var(txn.path) -m beg /a/b }
		// use_backend service-a   if { var(txn.host) example } { var(txn.path) -m beg /a }
		sort.Strings(sortedKeys)
		c.Client.BackendSwitchingRuleDeleteAll(frontend.Name)
		for _, key := range sortedKeys {
			rule := useBackendRules[key]
			var condTest string
			switch frontend.Mode {
			case "http":
				if rule.Host != "" {
					//TODO: provide option to do strict host matching
					condTest = fmt.Sprintf("{ var(txn.host) %s } ", rule.Host)
				}
				if rule.Path != "" {
					if rule.ExactMatch {
						condTest = fmt.Sprintf("%s{ var(txn.path) %s }", condTest, rule.Path)
					} else {
						condTest = fmt.Sprintf("%s{ var(txn.path) -m beg %s }", condTest, rule.Path)
					}
				}
				if condTest == "" {
					logger.Infof("both Host and Path are empty for frontend %v with backend %v, SKIP", frontend, rule.Backend)
					continue
				}
			case "tcp":
				if rule.Host == "" {
					logger.Infof("Empty SNI for backend %s, SKIP", rule.Backend)
					continue
				}
				condTest = fmt.Sprintf("{ req_ssl_sni -i %s } ", rule.Host)
			}
			err := c.Client.BackendSwitchingRuleCreate(frontend.Name, models.BackendSwitchingRule{
				Cond:     "if",
				CondTest: condTest,
				Name:     rule.Backend,
				Index:    utils.PtrInt64(0),
			})
			logger.Panic(err)
		}
		reload = true
		delete(c.cfg.BackendSwitchingModified, frontend.Name)
	}
	reload = c.clearBackends(activeBackends) || reload
	return reload
}

// Remove unused backends
func (c *HAProxyController) clearBackends(activeBackends map[string]struct{}) (reload bool) {
	allBackends, err := c.Client.BackendsGet()
	if err != nil {
		return false
	}
	for _, backend := range allBackends {
		if _, ok := activeBackends[backend.Name]; !ok {
			logger.Debugf("Deleting backend '%s'", backend.Name)
			if err := c.Client.BackendDelete(backend.Name); err != nil {
				logger.Panic(err)
			}
			reload = true
		}
	}
	return reload
}

func (c *HAProxyController) setDefaultBackend(backendName string) (err error) {
	for _, frontendName := range []string{FrontendHTTP, FrontendHTTPS} {
		frontend, e := c.Client.FrontendGet(frontendName)
		if e == nil {
			frontend.DefaultBackend = backendName
			e = c.Client.FrontendEdit(frontend)
		}
		if e != nil {
			err = e
		}
	}
	return err
}
