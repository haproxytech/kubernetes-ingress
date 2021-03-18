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
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type Routes struct {
	http           []*Route
	tcp            []*Route
	httpDefault    []*Route
	activeBackends map[string]struct{}
	reload         bool
}

var logger = utils.GetLogger()
var client api.HAProxyClient
var k8sStore store.K8s
var haproxyCerts *haproxy.Certificates

//nolint:golint,stylecheck
const (
	// Configmaps
	Main = "main"
	// Frontends
	FrontendHTTP  = "http"
	FrontendHTTPS = "https"
	// Status
	ADDED    = store.ADDED
	DELETED  = store.DELETED
	ERROR    = store.ERROR
	EMPTY    = store.EMPTY
	MODIFIED = store.MODIFIED
	// MapFiles
	SNI         = "sni"
	HOST        = "host"
	PATH_EXACT  = "path-exact"
	PATH_PREFIX = "path-prefix"
)

func (r *Routes) AddRoute(route *Route) (err error) {
	if route.BackendName == "" {
		err = route.SetBackendName()
		if err != nil {
			return err
		}
	}
	route.setStatus()
	switch {
	case route.Path.IsDefaultBackend:
		r.httpDefault = append([]*Route{route}, r.httpDefault...)
	case route.TCPService:
		r.tcp = append(r.tcp, route)
	default:
		r.http = append(r.http, route)
	}
	return nil
}

func (r *Routes) Refresh(c api.HAProxyClient, k store.K8s, mapFiles haproxy.Maps, certs *haproxy.Certificates) (reload bool, activeBackends map[string]struct{}) {
	client = c
	k8sStore = k
	haproxyCerts = certs
	r.activeBackends = make(map[string]struct{})
	logger.Trace("Updating Backend Switching rules")
	r.refreshHTTP(mapFiles)
	r.refreshHTTPDefault()
	r.refreshTCP()
	return r.reload, r.activeBackends
}

func (r *Routes) refreshHTTP(mapFiles haproxy.Maps) {
	for _, route := range r.http {
		// DELETED Route
		if route.status == DELETED {
			r.reload = true
			continue
		}
		if !route.LocalBackend {
			if err := route.handleBackend(); err != nil {
				logger.Error(err)
				continue
			}
			route.handleEndpoints()
		}
		r.reload = route.reload || r.reload
		err := route.addToMapFile(mapFiles)
		if err != nil {
			logger.Error(err)
		} else {
			r.activeBackends[route.BackendName] = struct{}{}
		}
	}
}

func (r *Routes) refreshHTTPDefault() {
	defaultBackend := ""
	// pick latest pushed default route
	for _, route := range r.httpDefault {
		if route.status != DELETED {
			err := route.handleBackend()
			if err != nil {
				logger.Error(err)
				continue
			}
			route.handleEndpoints()
			defaultBackend = route.BackendName
			r.activeBackends[route.BackendName] = struct{}{}
			break
		}
	}
	if frontend, err := client.FrontendGet(FrontendHTTP); err != nil {
		logger.Error(err)
		return
	} else if frontend.DefaultBackend == defaultBackend {
		return
	}
	if defaultBackend == "" {
		logger.Info("No default backend for http/https traffic")
	} else {
		logger.Infof("Setting http/https default backend to '%s'", defaultBackend)
	}
	for _, frontendName := range []string{FrontendHTTP, FrontendHTTPS} {
		frontend, _ := client.FrontendGet(frontendName)
		frontend.DefaultBackend = defaultBackend
		err := client.FrontendEdit(frontend)
		if err != nil {
			logger.Error(err)
			return
		}
	}
	r.reload = true
}

func (r *Routes) refreshTCP() {
	for _, route := range r.tcp {
		if route.status == DELETED {
			continue
		}
		if err := route.handleBackend(); err != nil {
			logger.Error(err)
			continue
		}
		route.handleEndpoints()
		r.reload = route.reload || r.reload
		r.activeBackends[route.BackendName] = struct{}{}
	}
}
