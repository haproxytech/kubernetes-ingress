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

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type IngressRoute struct {
	Namespace   *store.Namespace
	Path        *store.IngressPath
	Service     *store.Service
	Ingress     *store.Ingress
	Host        string
	BackendName string
	NewBackend  bool
}

// handleRoute processes an IngressRoute and make corresponding HAProxy configuration
// which results in the configuration of a Backend section + backend switching rules in Frontend.
func (c *HAProxyController) handleRoute(route *IngressRoute) (reload bool) {
	route.Service = route.Namespace.Services[route.Path.ServiceName]
	if route.Service == nil {
		logger.Warningf("ingress %s/%s: service '%s' not found", route.Namespace.Name, route.Ingress.Name, route.Path.ServiceName)
		return false
	}
	r, err := c.handleService(route)
	reload = reload || r
	if err != nil {
		logger.Error(err)
		return reload
	}
	if route.Path.Status == DELETED {
		return reload
	}
	return c.handleEndpoints(route)
}

// handleService processes an IngressRoute and make corresponding backend configuration in HAProxy
func (c *HAProxyController) handleService(route *IngressRoute) (reload bool, err error) {

	// Get Backend status
	status := route.Service.Status
	if status == EMPTY {
		status = route.Path.Status
	}

	// If status DELETED
	// remove use_backend rule and leave.
	// Backend will be deleted when no more use_backend
	// rules are left for the backend in question.
	// This is done via c.refreshBackendSwitching
	if status == DELETED {
		key := fmt.Sprintf("%s-%s-%s-%s", route.Host, route.Path.Path, route.Namespace.Name, route.Ingress.Name)
		switch {
		case route.Path.IsSSLPassthrough:
			c.deleteUseBackendRule(key, FrontendSSL)
		case route.Path.IsDefaultBackend:
			logger.Debugf("Removing default backend '%s/%s'", route.Namespace.Name, route.Service.Name)
			err = c.setDefaultBackend("")
			reload = true
		default:
			c.deleteUseBackendRule(key, FrontendHTTP, FrontendHTTPS)
		}
		return reload, err
	}

	// Set backendName
	if route.Path.ServicePortInt == 0 {
		route.BackendName = fmt.Sprintf("%s-%s-%s", route.Namespace.Name, route.Service.Name, route.Path.ServicePortString)
	} else {
		route.BackendName = fmt.Sprintf("%s-%s-%d", route.Namespace.Name, route.Service.Name, route.Path.ServicePortInt)
	}

	// Get/Create Backend
	reload = false
	var backend models.Backend
	if backend, err = c.Client.BackendGet(route.BackendName); err != nil {
		mode := "http"
		backend = models.Backend{
			Name: route.BackendName,
			Mode: mode,
		}
		if route.Path.IsTCPService || route.Path.IsSSLPassthrough {
			backend.Mode = string(TCP)
		}
		logger.Debugf("Ingress '%s/%s': Creating new backend '%s'", route.Namespace.Name, route.Ingress.Name, route.BackendName)
		if err = c.Client.BackendCreate(backend); err != nil {
			return reload, err
		}
		route.NewBackend = true
		reload = true
	}

	// handle Annotations
	activeSSLPassthrough := c.handleSSLPassthrough(route, &backend)
	activeBackendAnn := c.handleBackendAnnotations(route, &backend)
	if activeBackendAnn || activeSSLPassthrough {
		logger.Debugf("Ingress '%s/%s': Applying annotations changes to backend '%s'", route.Namespace.Name, route.Ingress.Name, route.BackendName)
		if err = c.Client.BackendEdit(backend); err != nil {
			return reload, err
		}
		reload = true
	}

	// No need to update BackendSwitching
	if (status == EMPTY && !activeSSLPassthrough) || route.Path.IsTCPService {
		return reload, nil
	}

	// Update backendSwitching
	key := fmt.Sprintf("%s-%s-%s-%s", route.Host, route.Path.Path, route.Namespace.Name, route.Ingress.Name)
	useBackendRule := UseBackendRule{
		Host:       route.Host,
		Path:       route.Path.Path,
		ExactMatch: route.Path.ExactPathMatch,
		Backend:    route.BackendName,
		Namespace:  route.Namespace.Name,
	}
	switch {
	case route.Path.IsDefaultBackend:
		logger.Debugf("Using service '%s/%s' as default backend", route.Namespace.Name, route.Service.Name)
		err = c.setDefaultBackend(route.BackendName)
		reload = true
	case route.Path.IsSSLPassthrough:
		c.addUseBackendRule(key, useBackendRule, FrontendSSL)
		if activeSSLPassthrough {
			c.deleteUseBackendRule(key, FrontendHTTP, FrontendHTTPS)
		}
	default:
		c.addUseBackendRule(key, useBackendRule, FrontendHTTP, FrontendHTTPS)
		if activeSSLPassthrough {
			c.deleteUseBackendRule(key, FrontendSSL)
		}
	}

	if err != nil {
		return reload, err
	}

	return reload, nil
}
