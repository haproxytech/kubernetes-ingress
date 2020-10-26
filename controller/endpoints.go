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
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

// alignSrvSlots adds or removes server slots in maint mode (disabled) to match servers-slots param
func (c *HAProxyController) alignSrvSlots(endpoints *store.Endpoints) (reload bool) {
	// Get server slots param
	// "servers-increment" is legacy annotation
	annServerSlots, _ := c.Store.GetValueFromAnnotations("servers-increment", c.Store.ConfigMaps[Main].Annotations)
	if annServerSlots == nil {
		annServerSlots, _ = c.Store.GetValueFromAnnotations("servers-slots", c.Store.ConfigMaps[Main].Annotations)

	}
	serverSlots := int(42)
	if value, err := strconv.Atoi(annServerSlots.Value); err == nil {
		serverSlots = value
	}
	// Add disabled HAProxySrvs to match serverSlots param
	srvName := ""
	for serverSlots-len(endpoints.HAProxySrvs) > 0 {
		srvName = fmt.Sprintf("SRV_%s", utils.RandomString(5))
		endpoints.HAProxySrvs[srvName] = &store.HAProxySrv{
			IP:       "127.0.0.1",
			Disabled: true,
			Modified: true,
		}
	}
	if srvName != "" {
		return true
	}
	// Remove disabled HAProxySlots if any to match serverSlots param
	var disabledSrv []string
	for srvName, slot := range endpoints.HAProxySrvs {
		if slot.Disabled {
			disabledSrv = append(disabledSrv, srvName)
		}
	}
	if disabledSrv == nil {
		return false
	}
	i := 0
	srvName = ""
	for serverSlots < len(endpoints.HAProxySrvs) && len(disabledSrv[i:]) > 0 {
		srvName = disabledSrv[i]
		logger.Debugf("Deleting server '%s/%s'", endpoints.BackendName, srvName)
		errAPI := c.Client.BackendServerDelete(endpoints.BackendName, srvName)
		if errAPI != nil {
			logger.Error(errAPI)
			return
		}
		delete(endpoints.HAProxySrvs, srvName)
		i++
	}
	return srvName != ""
}

// createSrvSlots add server slots for new Addresses with no available slots.
func (c *HAProxyController) createSrvSlots(endpoints *store.Endpoints) (reload bool) {
	// Get a list of addresses with no servers slots
	addresses := make(map[string]struct{}, len(endpoints.Addresses))
	for k, v := range endpoints.Addresses {
		addresses[k] = v
	}
	for _, slot := range endpoints.HAProxySrvs {
		delete(addresses, slot.IP)
	}
	// Create servers slots
	adr := ""
	for adr = range addresses {
		reload = true
		srvName := fmt.Sprintf("SRV_%s", utils.RandomString(5))
		endpoints.HAProxySrvs[srvName] = &store.HAProxySrv{
			IP:       adr,
			Disabled: false,
			Modified: true,
		}
	}
	if adr != "" {
		reload = true
	}
	return reload
}

// handleEndpoints lookups the IngressPath related endpoints and makes corresponding backend servers configuration in HAProxy
// If only the address changes , no need to reload just generate new config
func (c *HAProxyController) handleEndpoints(namespace *store.Namespace, ingress *store.Ingress, path *store.IngressPath, service *store.Service, backendName string, newBackend bool) (reload bool) {
	reload = newBackend
	// fetch Endpoints
	endpoints, ok := namespace.Endpoints[service.Name]
	if !ok {
		if service.DNS == "" {
			logger.Warningf("No Endpoints for service '%s'", service.Name)
			return false // not an end of world scenario, just log this
		}
		//TODO: currently HAProxy will only resolve server name at startup/reload
		// This needs to be improved by using HAPorxy resolvers to have resolution at runtime
		logger.Debugf("Configuring service '%s', of type ExternalName", service.Name)
		endpoints = &store.Endpoints{
			Namespace: "external",
			HAProxySrvs: map[string]*store.HAProxySrv{
				"external-service": &store.HAProxySrv{
					IP:       service.DNS,
					Disabled: false,
					Modified: true,
				},
			},
		}
		namespace.Endpoints[service.Name] = endpoints
	}
	endpoints.BackendName = backendName
	// resolve TargetPort
	portUpdated, err := c.setTargetPort(path, service, endpoints)
	reload = reload || portUpdated
	if err != nil {
		logger.Error(err)
		return false
	}
	// Handle Backend servers
	if len(endpoints.HAProxySrvs) < len(endpoints.Addresses) {
		reload = c.createSrvSlots(endpoints) || reload
	}
	reload = c.alignSrvSlots(endpoints) || reload
	annotations, activeAnnotations := c.getServerAnnotations(ingress, service)
	reload = reload || activeAnnotations
	for srvName, srv := range endpoints.HAProxySrvs {
		if !srv.Modified && !reload {
			continue
		}
		c.handleHAProxSrv(endpoints, srvName, path.TargetPort, annotations)
	}
	return reload
}

// handleHAProxSrv creates/updates corresponding HAProxy backend server
func (c *HAProxyController) handleHAProxSrv(endpoints *store.Endpoints, srvName string, port int64, annotations map[string]*store.StringW) {
	srv, ok := endpoints.HAProxySrvs[srvName]
	if !ok {
		return
	}
	server := models.Server{
		Name:    srvName,
		Address: srv.IP,
		Port:    &port,
		Weight:  utils.PtrInt64(128),
	}
	if srv.Disabled {
		server.Maintenance = "enabled"
	}
	c.handleServerAnnotations(&server, annotations)
	errAPI := c.Client.BackendServerEdit(endpoints.BackendName, server)
	if errAPI == nil {
		logger.Debugf("Updating server '%s/%s'", endpoints.BackendName, server.Name)
		return
	}
	if strings.Contains(errAPI.Error(), "does not exist") {
		logger.Debugf("Creating server '%s/%s'", endpoints.BackendName, server.Name)
		logger.Error(c.Client.BackendServerCreate(endpoints.BackendName, server))
	}
}

// setTargetPort looks for the targetPort (Endpoint port) corresponding to the servicePort of the IngressPath
func (c *HAProxyController) setTargetPort(path *store.IngressPath, service *store.Service, endpoints *store.Endpoints) (update bool, err error) {
	//ExternalName
	if path.ServicePortInt != 0 && endpoints.Namespace == "external" {
		if path.TargetPort != path.ServicePortInt {
			path.TargetPort = path.ServicePortInt
			update = true
		}
		return update, nil
	}
	// Ingress.ServicePort lookup: Ingress.ServicePort --> Service.Port
	for _, sp := range service.Ports {
		if sp.Name == path.ServicePortString || sp.Port == path.ServicePortInt {
			if endpoints != nil {
				// Service.Port lookup: Service.Port --> Endpoints.Port
				if targetPort, ok := endpoints.Ports[sp.Name]; ok {
					if path.TargetPort != targetPort {
						path.TargetPort = targetPort
						update = true
					}
					return update, nil
				}
			}
			return update, fmt.Errorf("Could not find '%s' Targetport for service '%s'", sp.Name, service.Name)
		}
	}
	return update, fmt.Errorf("ingress servicePort(Str: %s, Int: %d) not found for backend '%s'", path.ServicePortString, path.ServicePortInt, endpoints.BackendName)
}

// processEndpointsSrvs dynamically update (via runtime socket) HAProxy backend servers with modified Addresses
func (c *HAProxyController) processEndpointsSrvs(oldEndpoints, newEndpoints *store.Endpoints) {
	// Compare new Endpoints with old Endpoints Addresses and sync HAProxySrvs
	// Also by the end we will have a temporary array holding available HAProxysrv slots
	available := []*store.HAProxySrv{}
	newEndpoints.HAProxySrvs = oldEndpoints.HAProxySrvs
	for _, srv := range newEndpoints.HAProxySrvs {
		if _, ok := newEndpoints.Addresses[srv.IP]; !ok {
			available = append(available, srv)
			if !srv.Disabled {
				srv.IP = "127.0.0.1"
				srv.Disabled = true
				srv.Modified = true
			}
		}
	}
	// Check available HAProxySrvs to add new Addreses
	availableIdx := len(available) - 1
	for newAdr := range newEndpoints.Addresses {
		if availableIdx < 0 {
			break
		}
		if _, ok := oldEndpoints.Addresses[newAdr]; !ok {
			srv := available[availableIdx]
			srv.IP = newAdr
			srv.Disabled = false
			srv.Modified = true
			available = available[:availableIdx]
			availableIdx--
		}
	}
	// Dynamically updates HAProxy backend servers  with HAProxySrvs content
	for srvName, srv := range newEndpoints.HAProxySrvs {
		if srv.Modified {
			if newEndpoints.BackendName == "" {
				logger.Errorf("No backend Name for endpoints of service `%s` ", newEndpoints.Service.Value)
				break
			}
			logger.Error(c.Client.SetServerAddr(newEndpoints.BackendName, srvName, srv.IP, 0))
			status := "ready"
			if srv.Disabled {
				status = "maint"
			}
			logger.Debugf("server '%s/%s' changed status to %v", newEndpoints.BackendName, srvName, status)
			logger.Error(c.Client.SetServerState(newEndpoints.BackendName, srvName, status))
		}
	}
}
