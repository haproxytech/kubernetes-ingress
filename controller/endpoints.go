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

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

// alignSrvSlots adds or removes server slots in maint mode (disabled) to match servers-slots param
func (c *HAProxyController) alignSrvSlots(endpoints *Endpoints) {
	// Get server slots param
	// "servers-increment" is legacy annotation
	annServerSlots, _ := GetValueFromAnnotations("servers-increment", c.cfg.ConfigMaps[Main].Annotations)
	if annServerSlots == nil {
		annServerSlots, _ = GetValueFromAnnotations("servers-slots", c.cfg.ConfigMaps[Main].Annotations)

	}
	serverSlots := int(42)
	if value, err := strconv.Atoi(annServerSlots.Value); err == nil {
		serverSlots = value
	}
	// Add disabled HAProxySrvs to match serverSlots param
	for serverSlots-len(endpoints.HAProxySrvs) > 0 {
		srvName := fmt.Sprintf("SRV_%s", utils.RandomString(5))
		endpoints.HAProxySrvs[srvName] = &HAProxySrv{
			IP:       "127.0.0.1",
			Disabled: true,
			Modified: true,
		}
	}
	// Remove disabled HAProxySlots if any to match serverSlots param
	var disabledSrv []string
	for srvName, slot := range endpoints.HAProxySrvs {
		if slot.Disabled {
			disabledSrv = append(disabledSrv, srvName)
		}
	}
	if disabledSrv == nil {
		return
	}
	i := 0
	for serverSlots < len(endpoints.HAProxySrvs) && len(disabledSrv[i:]) > 0 {
		srvName := disabledSrv[i]
		c.Logger.Debugf("Deleting server '%s/%s'", endpoints.BackendName, srvName)
		errAPI := c.Client.BackendServerDelete(endpoints.BackendName, srvName)
		if errAPI != nil {
			c.Logger.Error(errAPI)
			return
		}
		delete(endpoints.HAProxySrvs, srvName)
		i++
	}
}

// createSrvSlots add server slots for new Addresses with no available slots.
func (c *HAProxyController) createSrvSlots(endpoints *Endpoints) {
	// Get a list of addresses with no servers slots
	addresses := make(map[string]struct{}, len(endpoints.Addresses))
	for k, v := range endpoints.Addresses {
		addresses[k] = v
	}
	for _, slot := range endpoints.HAProxySrvs {
		delete(addresses, slot.IP)
	}
	// Create servers slots
	for adr := range addresses {
		srvName := fmt.Sprintf("SRV_%s", utils.RandomString(5))
		endpoints.HAProxySrvs[srvName] = &HAProxySrv{
			IP:       adr,
			Disabled: false,
			Modified: true,
		}
	}
}

// handleEndpoints lookups the IngressPath related endpoints and makes corresponding backend servers configuration in HAProxy
func (c *HAProxyController) handleEndpoints(namespace *Namespace, ingress *Ingress, path *IngressPath, service *Service, backendName string, newBackend bool) (reload bool) {
	// fetch Endpoints
	endpoints, ok := namespace.Endpoints[service.Name]
	if !ok {
		if service.DNS == "" {
			c.Logger.Warningf("No Endpoints for service '%s'", service.Name)
			return false // not an end of world scenario, just log this
		}
		//TODO: currently HAProxy will only resolve server name at startup/reload
		// This needs to be improved by using HAPorxy resolvers to have resolution at runtime
		c.Logger.Debugf("Configuring service '%s', of type ExternalName", service.Name)
		endpoints = &Endpoints{
			Namespace: "external",
			HAProxySrvs: map[string]*HAProxySrv{
				"external-service": &HAProxySrv{
					IP:       service.DNS,
					Disabled: false,
					Modified: true,
				},
			},
		}
		namespace.Endpoints[service.Name] = endpoints
	}
	// resolve TargetPort
	endpoints.BackendName = backendName
	if err := c.setTargetPort(path, service, endpoints); err != nil {
		c.Logger.Error(err)
		return false
	}
	// Handle Backend servers
	if len(endpoints.HAProxySrvs) < len(endpoints.Addresses) {
		c.createSrvSlots(endpoints)
	}
	c.alignSrvSlots(endpoints)
	annotations, activeAnnotations := c.getServerAnnotations(ingress, service)
	for srvName, srv := range endpoints.HAProxySrvs {
		if !newBackend && !srv.Modified && !activeAnnotations {
			continue
		}
		reload = c.handleEndpoint(endpoints, srvName, path.TargetPort, annotations) || reload
	}
	return reload
}

// handleEndpoint creates/updates corresponding HAProxy backend server
func (c *HAProxyController) handleEndpoint(endpoints *Endpoints, srvName string, port int64, annotations map[string]*StringW) (reload bool) {
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
		c.Logger.Debugf("Updating server '%s/%s'", endpoints.BackendName, server.Name)
		return true
	}
	if strings.Contains(errAPI.Error(), "does not exist") {
		c.Logger.Debugf("Creating server '%s/%s'", endpoints.BackendName, server.Name)
		errAPI = c.Client.BackendServerCreate(endpoints.BackendName, server)
		if errAPI != nil {
			c.Logger.Err(errAPI)
			return false
		}
	}
	return true
}

// setTargetPort looks for the targetPort (Endpoint port) corresponding to the servicePort of the IngressPath
func (c *HAProxyController) setTargetPort(path *IngressPath, service *Service, endpoints *Endpoints) error {
	//ExternalName
	if path.ServicePortInt != 0 && endpoints.Namespace == "external" {
		path.TargetPort = path.ServicePortInt
		return nil
	}
	// Ingress.ServicePort lookup: Ingress.ServicePort --> Service.Port
	for _, sp := range service.Ports {
		if sp.Name == path.ServicePortString || sp.Port == path.ServicePortInt {
			// Service.Port lookup: Service.Port --> Endpoints.Port
			if endpoints != nil {
				if targetPort, ok := endpoints.Ports[sp.Name]; ok {
					if path.TargetPort != targetPort && path.TargetPort != 0 {
						// Dinamically update backend port
						for srvName, srv := range endpoints.HAProxySrvs {
							if err := c.Client.SetServerAddr(endpoints.BackendName, srvName, srv.IP, int(targetPort)); err != nil {
								c.Logger.Error(err)
							}
							c.Logger.Infof("TargetPort of backend '%s' changed from %d to %d", endpoints.BackendName, path.TargetPort, targetPort)
						}
					}
					path.TargetPort = targetPort
					return nil
				}
			}
			c.Logger.Warningf("Could not find '%s' Targetport for service '%s'", sp.Name, service.Name)
			return nil
		}
	}
	return fmt.Errorf("ingress servicePort(Str: %s, Int: %d) not found for backend '%s'", path.ServicePortString, path.ServicePortInt, endpoints.BackendName)
}

// processEndpointsSrvs dynamically update (via runtime socket) HAProxy backend servers with modified Addresses
func (c *HAProxyController) processEndpointsSrvs(oldEndpoints, newEndpoints *Endpoints) {
	// Compare new Endpoints with old Endpoints Addresses and sync HAProxySrvs
	// Also by the end we will have a temporary array holding available HAProxysrv slots
	available := []*HAProxySrv{}
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
				c.Logger.Errorf("No backend Name for endpoints of service `%s` ", newEndpoints.Service.Value)
				break
			}
			c.Logger.Error(c.Client.SetServerAddr(newEndpoints.BackendName, srvName, srv.IP, 0))
			status := "ready"
			if srv.Disabled {
				status = "maint"
			}
			c.Logger.Debugf("server '%s/%s' changed status to %v", newEndpoints.BackendName, srvName, status)
			c.Logger.Error(c.Client.SetServerState(newEndpoints.BackendName, srvName, status))
		}
	}
}
