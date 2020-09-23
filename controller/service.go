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
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/models/v2"
)

// handle defaultBackned configured via cli param "default-backend-service"
func (c *HAProxyController) handleDefaultService() (reload bool) {
	dsvcData, _ := c.Store.GetValueFromAnnotations("default-backend-service")
	dsvc := strings.Split(dsvcData.Value, "/")

	if len(dsvc) != 2 {
		c.Logger.Errorf("default service invalid data")
		return false
	}
	if dsvc[0] == "" || dsvc[1] == "" {
		return false
	}
	namespace, ok := c.Store.Namespaces[dsvc[0]]
	if !ok {
		c.Logger.Errorf("default service invalid namespace " + dsvc[0])
		return false
	}
	service, ok := namespace.Services[dsvc[1]]
	if !ok {
		c.Logger.Errorf("service '" + dsvc[1] + "' does not exist")
		return false
	}
	ingress := &store.Ingress{
		Namespace:   namespace.Name,
		Name:        "DefaultService",
		Annotations: store.MapStringW{},
		Rules:       map[string]*store.IngressRule{},
	}
	path := &store.IngressPath{
		ServiceName:      service.Name,
		ServicePortInt:   service.Ports[0].Port,
		IsDefaultBackend: true,
	}
	return c.handlePath(namespace, ingress, &store.IngressRule{}, path)
}

// handle service of an IngressPath and make corresponding backend configuration in HAProxy
func (c *HAProxyController) handleService(namespace *store.Namespace, ingress *store.Ingress, rule *store.IngressRule, path *store.IngressPath, service *store.Service) (backendName string, newBackend bool, reload bool, err error) {

	// Get Backend status
	status := service.Status
	if status == EMPTY {
		status = path.Status
	}

	// If status DELETED
	// remove use_backend rule and leave.
	// Backend will be deleted when no more use_backend
	// rules are left for the backend in question.
	// This is done via c.refreshBackendSwitching
	if status == DELETED {
		key := fmt.Sprintf("%s-%s-%s-%s", rule.Host, path.Path, namespace.Name, ingress.Name)
		switch {
		case path.IsSSLPassthrough:
			c.deleteUseBackendRule(key, FrontendSSL)
		case path.IsDefaultBackend:
			c.Logger.Debugf("Removing default backend '%s/%s'", namespace.Name, service.Name)
			err = c.setDefaultBackend("")
			reload = true
		default:
			c.deleteUseBackendRule(key, FrontendHTTP, FrontendHTTPS)
		}
		return "", false, reload, err
	}

	// Set backendName
	if path.ServicePortInt == 0 {
		backendName = fmt.Sprintf("%s-%s-%s", namespace.Name, service.Name, path.ServicePortString)
	} else {
		backendName = fmt.Sprintf("%s-%s-%d", namespace.Name, service.Name, path.ServicePortInt)
	}

	// Get/Create Backend
	newBackend = false
	reload = false
	var backend models.Backend
	if backend, err = c.Client.BackendGet(backendName); err != nil {
		mode := "http"
		backend = models.Backend{
			Name: backendName,
			Mode: mode,
		}
		if path.IsTCPService || path.IsSSLPassthrough {
			backend.Mode = string(TCP)
		}
		c.Logger.Debugf("Ingress '%s/%s': Creating new backend '%s'", namespace.Name, ingress.Name, backendName)
		if err = c.Client.BackendCreate(backend); err != nil {
			return "", true, reload, err
		}
		newBackend = true
		reload = true
	}

	// handle Annotations
	activeSSLPassthrough := c.handleSSLPassthrough(ingress, service, path, &backend, newBackend)
	activeBackendAnn := c.handleBackendAnnotations(ingress, service, &backend, newBackend)
	if activeBackendAnn || activeSSLPassthrough {
		c.Logger.Debugf("Ingress '%s/%s': Applying annotations changes to backend '%s'", namespace.Name, ingress.Name, backendName)
		if err = c.Client.BackendEdit(backend); err != nil {
			return backendName, newBackend, reload, err
		}
		reload = true
	}

	// No need to update BackendSwitching
	if (status == EMPTY && !activeSSLPassthrough) || path.IsTCPService {
		return backendName, newBackend, reload, nil
	}

	// Update backendSwitching
	key := fmt.Sprintf("%s-%s-%s-%s", rule.Host, path.Path, namespace.Name, ingress.Name)
	useBackendRule := UseBackendRule{
		Host:      rule.Host,
		Path:      path.Path,
		Backend:   backendName,
		Namespace: namespace.Name,
	}
	switch {
	case path.IsDefaultBackend:
		c.Logger.Debugf("Using service '%s/%s' as default backend", namespace.Name, service.Name)
		err = c.setDefaultBackend(backendName)
		reload = true
	case path.IsSSLPassthrough:
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
		return "", newBackend, reload, err
	}

	return backendName, newBackend, reload, nil
}

// handle IngressPath and make corresponding HAProxy configuration
func (c *HAProxyController) handlePath(namespace *store.Namespace, ingress *store.Ingress, rule *store.IngressRule, path *store.IngressPath) (reload bool) {
	// fetch Service
	service, ok := namespace.Services[path.ServiceName]
	if !ok {
		c.Logger.Errorf("service '%s' does not exist", path.ServiceName)
		return false
	}
	// handle backend
	backendName, newBackend, r, err := c.handleService(namespace, ingress, rule, path, service)
	reload = reload || r
	if err != nil {
		c.Logger.Error(err)
		return reload
	}
	if path.Status == DELETED {
		return reload
	}
	// handle backend servers
	return c.handleEndpoints(namespace, ingress, path, service, backendName, newBackend)
}

// handle pprof backend
func (c *HAProxyController) handlePprof() (err error) {
	pprofBackend := "pprof"

	err = c.Client.BackendCreate(models.Backend{
		Name: pprofBackend,
		Mode: "http",
	})
	if err != nil {
		return err
	}
	err = c.Client.BackendServerCreate(pprofBackend, models.Server{
		Name:    "pprof",
		Address: "127.0.0.1:6060",
	})
	if err != nil {
		return err
	}
	c.Logger.Debug("pprof backend created")
	useBackendRule := UseBackendRule{
		Host:    "",
		Path:    "/debug/pprof",
		Backend: pprofBackend,
	}
	c.addUseBackendRule("pprof", useBackendRule, FrontendHTTPS)
	return nil
}
