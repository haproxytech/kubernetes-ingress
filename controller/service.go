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

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

// handle defaultBackned configured via cli param "default-backend-service"
func (c *HAProxyController) handleDefaultService() (reload bool, err error) {
	reload = false
	dsvcData, _ := GetValueFromAnnotations("default-backend-service")
	dsvc := strings.Split(dsvcData.Value, "/")

	if len(dsvc) != 2 {
		return reload, fmt.Errorf("default service invalid data")
	}
	if dsvc[0] == "" || dsvc[1] == "" {
		return reload, nil
	}
	namespace, ok := c.cfg.Namespace[dsvc[0]]
	if !ok {
		return reload, fmt.Errorf("default service invalid namespace " + dsvc[0])
	}
	service, ok := namespace.Services[dsvc[1]]
	if !ok {
		return reload, fmt.Errorf("service '" + dsvc[1] + "' does not exist")
	}
	ingress := &Ingress{
		Namespace:   namespace.Name,
		Name:        "DefaultService",
		Annotations: MapStringW{},
		Rules:       map[string]*IngressRule{},
	}
	path := &IngressPath{
		ServiceName:      service.Name,
		ServicePortInt:   service.Ports[0].Port,
		IsDefaultBackend: true,
	}
	return c.handlePath(namespace, ingress, &IngressRule{}, path)
}

// handle the IngressPath related endpoints and make corresponding backend servers configuration in HAProxy
func (c *HAProxyController) handleEndpoint(namespace *Namespace, ingress *Ingress, path *IngressPath, service *Service, backendName string, ip EndpointIP) (reload bool) {
	server := models.Server{
		Name:    ip.HAProxyName,
		Address: ip.IP,
		Port:    &path.TargetPort,
		Weight:  utils.PtrInt64(128),
	}
	if ip.Disabled {
		server.Maintenance = "enabled"
	}

	annotationsActive := c.handleServerAnnotations(ingress, service, &server, ip.Status)
	if ip.Status == EMPTY && annotationsActive {
		ip.Status = MODIFIED
	}

	var errAPI error
	switch ip.Status {
	case ADDED, MODIFIED:
		errAPI = c.Client.BackendServerEdit(backendName, server)
		if errAPI != nil {
			if strings.Contains(errAPI.Error(), "does not exist") {
				c.Logger.Debugf("Creating server '%s/%s'", backendName, server.Name)
				errAPI = c.Client.BackendServerCreate(backendName, server)
				break
			}
			c.Logger.Debugf("Updating server '%s/%s'", backendName, server.Name)
		}

	case DELETED:
		c.Logger.Debugf("Deleting server '%s/%s'", backendName, server.Name)
		errAPI = c.Client.BackendServerDelete(backendName, server.Name)
	}
	if errAPI != nil {
		c.Logger.Err(errAPI)
		return false
	}
	return false
}

// handle service of an IngressPath and make corresponding backend configuration in HAProxy
func (c *HAProxyController) handleService(namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath, service *Service) (backendName string, newBackend bool, reload bool, err error) {

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
func (c *HAProxyController) handlePath(namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath) (reload bool, err error) {
	reload = false
	// fetch Service
	service, ok := namespace.Services[path.ServiceName]
	if !ok {
		return reload, fmt.Errorf("service '%s' does not exist", path.ServiceName)
	}
	// handle Backend
	backendName, newBackend, r, err := c.handleService(namespace, ingress, rule, path, service)
	reload = reload || r
	if err != nil {
		return reload, err
	}
	// fetch Endpoints
	endpoints, ok := namespace.Endpoints[service.Name]
	if !ok {
		if service.DNS == "" {
			c.Logger.Warningf("No Endpoints for service '%s'", service.Name)
			return reload, nil // not an end of world scenario, just log this
		}
		//TODO: currently HAProxy will only resolve server name at startup/reload
		// This needs to be improved by using HAPorxy resolvers to have resolution at runtime
		c.Logger.Debugf("Configuring service '%s', of type ExternalName", service.Name)
		endpoints = &Endpoints{
			Namespace: "external",
			Addresses: &EndpointIPs{
				"external": &EndpointIP{
					HAProxyName: "external-service",
					IP:          service.DNS,
					Disabled:    false,
					Status:      service.Status,
				},
			},
		}
	}
	// resolve TargetPort
	endpoints.BackendName = backendName
	if err := c.setTargetPort(path, service, endpoints); err != nil {
		return reload, err
	}
	// Handle Backend servers
	for _, endpoint := range *endpoints.Addresses {
		endpoint := *endpoint
		if newBackend {
			endpoint.Status = ADDED
		}
		r := c.handleEndpoint(namespace, ingress, path, service, backendName, endpoint)
		reload = reload || r
	}
	return reload, nil
}

// Look for the targetPort (Endpoint port) corresponding to the servicePort of the IngressPath
func (c *HAProxyController) setTargetPort(path *IngressPath, service *Service, endpoints *Endpoints) error {
	for _, sp := range service.Ports {
		// Find corresponding servicePort
		if sp.Name == path.ServicePortString || sp.Port == path.ServicePortInt {
			// Find the corresponding targetPort in Endpoints ports
			if endpoints != nil {
				for _, epPort := range *endpoints.Ports {
					if epPort.Name == sp.Name {
						// Dinamically update backend port
						if path.TargetPort != epPort.Port && path.TargetPort != 0 {
							for _, EndpointIP := range *endpoints.Addresses {
								if err := c.Client.SetServerAddr(endpoints.BackendName, EndpointIP.HAProxyName, EndpointIP.IP, int(epPort.Port)); err != nil {
									c.Logger.Error(err)
								}
								c.Logger.Infof("Servers Port of backend %s changed from %d to %d", endpoints.BackendName, path.TargetPort, epPort.Port)
							}
						}
						path.TargetPort = epPort.Port
						return nil
					}
				}
			}
		}
	}
	// TargetPort was not found
	if path.ServicePortInt != 0 && endpoints.Namespace == "external" {
		//ExternalName
		path.TargetPort = path.ServicePortInt
		return nil
	}
	return fmt.Errorf("ingress servicePort(Str: %s, Int: %d) for serviceName '%s' not found", path.ServicePortString, path.ServicePortInt, service.Name)
}
