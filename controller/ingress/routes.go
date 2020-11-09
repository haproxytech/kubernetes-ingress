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

package ingress

import (
	"fmt"
	"sort"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type Routes struct {
	http           map[string]*Route
	httpDefault    *Route
	sslPassthrough []*Route
	tcp            []*Route
}

var logger = utils.GetLogger()
var client api.HAProxyClient
var k8sStore store.K8s

const (
	// Configmaps
	Main = "main"
	//frontends
	FrontendHTTP  = "http"
	FrontendHTTPS = "https"
	FrontendSSL   = "ssl"
	//Status
	ADDED    = store.ADDED
	DELETED  = store.DELETED
	ERROR    = store.ERROR
	EMPTY    = store.EMPTY
	MODIFIED = store.MODIFIED
)

func NewRoutes() Routes {
	routes := Routes{
		http: make(map[string]*Route),
	}
	return routes
}

func (r *Routes) AddRoute(route *Route) {
	route.service = route.Namespace.Services[route.Path.ServiceName]
	if route.service == nil {
		logger.Warningf("ingress %s/%s: service '%s' not found", route.Namespace.Name, route.Ingress.Name, route.Path.ServiceName)
		return
	}
	route.setStatus()
	switch {
	case route.Path.IsDefaultBackend:
		r.httpDefault = route
	case route.TCPService:
		r.tcp = append(r.tcp, route)
	case route.SSLPassthrough:
		r.sslPassthrough = append(r.sslPassthrough, route)
	default:
		key := fmt.Sprintf("%s-%s-%s-%s", route.Host, route.Path.Path, route.Ingress.Name, route.Namespace.Name)
		r.http[key] = route
	}
}

func (r *Routes) Refresh(c api.HAProxyClient, k store.K8s) (reload bool, activeBackends map[string]struct{}) {
	client = c
	k8sStore = k
	activeBackends = make(map[string]struct{})

	logger.Debug("Updating Backend Switching rules")
	// Default Routes
	reload = r.refreshTCP(activeBackends) || reload
	reload = r.refreshHTTPDefault(activeBackends) || reload
	// SSLPassthrough Routes
	reload = r.refreshSSLPassthroug(activeBackends) || reload
	// HTTP Routes
	reload = r.refreshHTTP(activeBackends) || reload
	return reload, activeBackends
}

func (r *Routes) refreshHTTP(activeBackends map[string]struct{}) (reload bool) {
	sortedKeys := []string{}
	for key, route := range r.http {
		if route.status == DELETED {
			reload = true
			continue
		}
		// At this stage if backendName !="" then it was
		// manually configured to use a local backend.
		// For now this is only the case for Pprof
		if route.BackendName == "" {
			if err := route.handleService(); err != nil {
				logger.Error(err)
				continue
			}
			route.handleEndpoints()
		}
		activeBackends[route.BackendName] = struct{}{}
		sortedKeys = append(sortedKeys, key)
		reload = route.reload || reload
	}
	sort.Strings(sortedKeys)
	r.updateHTTPUseBackendRules(sortedKeys)
	return reload
}

func (r *Routes) updateHTTPUseBackendRules(sortedKeys []string) {
	// host/path are part of frontendRoutes keys, so sorted keys will
	// result in sorted use_backend rules where the longest path will match first.
	// Example:
	// use_backend service-abc if { var(txn.host) example } { var(txn.path) -m beg /a/b/c }
	// use_backend service-ab  if { var(txn.host) example } { var(txn.path) -m beg /a/b }
	// use_backend service-a   if { var(txn.host) example } { var(txn.path) -m beg /a }
	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS} {
		client.BackendSwitchingRuleDeleteAll(frontend)
		for _, key := range sortedKeys {
			route := r.http[key]
			var condTest string
			if route.Host != "" {
				condTest = fmt.Sprintf("{ var(txn.host) %s } ", route.Host)
			}
			if route.Path.Path != "" {
				if route.Path.ExactPathMatch {
					condTest = fmt.Sprintf("%s{ var(txn.path) %s }", condTest, route.Path.Path)
				} else {
					condTest = fmt.Sprintf("%s{ var(txn.path) -m beg %s }", condTest, route.Path.Path)
				}
			}
			if condTest == "" {
				logger.Infof("both Host and Path are empty for frontend %s with backend %s, SKIP", frontend, route.BackendName)
				continue
			}
			err := client.BackendSwitchingRuleCreate(frontend, models.BackendSwitchingRule{
				Cond:     "if",
				CondTest: condTest,
				Name:     route.BackendName,
				Index:    utils.PtrInt64(0),
			})
			logger.Error(err)
		}
	}
}

func (r *Routes) refreshSSLPassthroug(activeBackends map[string]struct{}) (reload bool) {
	client.BackendSwitchingRuleDeleteAll(FrontendSSL)
	for _, route := range r.sslPassthrough {
		if route.status == DELETED {
			reload = true
			continue
		}
		if err := route.handleService(); err != nil {
			logger.Error(err)
			continue
		}
		route.handleEndpoints()
		activeBackends[route.BackendName] = struct{}{}
		reload = route.reload || reload
		if route.Host == "" {
			logger.Infof("Empty SNI for backend %s, SKIP", route.BackendName)
			continue
		}
		err := client.BackendSwitchingRuleCreate(FrontendSSL, models.BackendSwitchingRule{
			Cond:     "if",
			CondTest: fmt.Sprintf("{ req_ssl_sni -i %s } ", route.Host),
			Name:     route.BackendName,
			Index:    utils.PtrInt64(0),
		})
		logger.Error(err)
	}
	return reload
}

func (r *Routes) refreshTCP(activeBackends map[string]struct{}) (reload bool) {
	for _, route := range r.tcp {
		err := route.handleService()
		if err != nil {
			logger.Error(err)
			continue
		}
		route.handleEndpoints()
		activeBackends[route.BackendName] = struct{}{}
		reload = route.reload || reload
	}
	return reload
}

func (r *Routes) refreshHTTPDefault(activeBackends map[string]struct{}) (reload bool) {
	if r.httpDefault == nil {
		return false
	}
	route := r.httpDefault
	activeBackends[route.BackendName] = struct{}{}
	if route.status != DELETED {
		logger.Debugf("Using service '%s/%s' as default backend", route.Namespace.Name, route.service.Name)
		err := route.handleService()
		if err != nil {
			logger.Error(err)
			return false
		}
		route.handleEndpoints()
	} else {
		logger.Debugf("Removing default backend '%s/%s'", route.Namespace.Name, route.service.Name)
		route.BackendName = ""
	}
	if route.status == EMPTY {
		return false
	}
	for _, frontendName := range []string{FrontendHTTP, FrontendHTTPS} {
		frontend, err := client.FrontendGet(frontendName)
		if err == nil {
			frontend.DefaultBackend = route.BackendName
			err = client.FrontendEdit(frontend)
		}
		if err != nil {
			logger.Error(err)
		} else {
			reload = true
		}
	}
	return reload
}
