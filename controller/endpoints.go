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

// alignHAproxySrvs adds or removes servers to match server-slots param
func (c *HAProxyController) alignHAproxySrvs(endpoints *store.Endpoints) (reload bool) {
	haproxySrvs := endpoints.HAProxySrvs
	// Get server-slots annotation
	// "servers-increment" is a legacy annotation
	annServerSlots, _ := c.Store.GetValueFromAnnotations("servers-increment", c.Store.ConfigMaps[Main].Annotations)
	if annServerSlots == nil {
		annServerSlots, _ = c.Store.GetValueFromAnnotations("server-slots", c.Store.ConfigMaps[Main].Annotations)

	}
	requiredSrvNbr := int(42)
	if value, err := strconv.Atoi(annServerSlots.Value); err == nil {
		requiredSrvNbr = value
	} else {
		logger.Error(err)
	}
	// Add disabled HAProxySrvs to match required serverSlots
	for len(haproxySrvs) < requiredSrvNbr {
		srvName := fmt.Sprintf("SRV_%s", utils.RandomString(5))
		haproxySrvs[srvName] = &store.HAProxySrv{
			Address:  "",
			Modified: true,
		}
		reload = true
	}
	// Remove HAProxySrvs to match required serverSlots
	for len(haproxySrvs) > requiredSrvNbr {
		// pick random server
		for srvName, srv := range haproxySrvs {
			srvAddr := srv.Address
			if _, ok := endpoints.AddrsUsed[srvAddr]; ok {
				delete(endpoints.AddrsUsed, srvAddr)
				endpoints.AddrRemain[srvAddr] = struct{}{}
			}
			delete(haproxySrvs, srvName)
			logger.Error(c.Client.BackendServerDelete(endpoints.BackendName, srvName))
			reload = true
			continue
		}
	}
	// Configure remaining addresses in available HAProxySrvs
	for _, srv := range haproxySrvs {
		if srv.Address == "" {
			for addr := range endpoints.AddrRemain {
				srv.Address = addr
				srv.Modified = true
				endpoints.AddrsUsed[addr] = struct{}{}
				delete(endpoints.AddrRemain, addr)
				break
			}
		}
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
		// This needs to be improved by using HAProxy resolvers to have resolution at runtime
		logger.Debugf("Configuring service '%s', of type ExternalName", service.Name)
		endpoints = &store.Endpoints{
			Namespace: "external",
			HAProxySrvs: map[string]*store.HAProxySrv{
				"external-service": {
					Address:  service.DNS,
					Modified: true,
				},
			},
		}
		namespace.Endpoints[service.Name] = endpoints
	}
	// resolve TargetPort
	portUpdated, err := c.setTargetPort(path, service, endpoints)
	reload = reload || portUpdated
	if err != nil {
		logger.Error(err)
		return false
	}
	// Handle Backend servers
	endpoints.BackendName = backendName
	annotations, activeAnnotations := c.getServerAnnotations(ingress, service)
	srvsNbrChanged := c.alignHAproxySrvs(endpoints)
	reload = reload || srvsNbrChanged || activeAnnotations
	for srvName, srv := range endpoints.HAProxySrvs {
		if srv.Modified || activeAnnotations {
			c.handleHAProxSrv(srvName, srv.Address, backendName, path.TargetPort, annotations)
		}
	}
	return reload
}

// handleHAProxSrv creates/updates corresponding HAProxy backend server
func (c *HAProxyController) handleHAProxSrv(srvName, srvAddr, backendName string, port int64, annotations map[string]*store.StringW) {
	server := models.Server{
		Name:    srvName,
		Address: srvAddr,
		Port:    &port,
		Weight:  utils.PtrInt64(128),
	}
	if server.Address == "" {
		server.Address = "127.0.0.1"
		server.Maintenance = "enabled"
	}
	c.handleServerAnnotations(&server, annotations)
	errAPI := c.Client.BackendServerEdit(backendName, server)
	if errAPI == nil {
		logger.Debugf("Updating server '%s/%s'", backendName, server.Name)
		return
	}
	if strings.Contains(errAPI.Error(), "does not exist") {
		logger.Debugf("Creating server '%s/%s'", backendName, server.Name)
		logger.Error(c.Client.BackendServerCreate(backendName, server))
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
			return update, fmt.Errorf("could not find '%s' Targetport for service '%s'", sp.Name, service.Name)
		}
	}
	return update, fmt.Errorf("ingress servicePort(Str: %s, Int: %d) not found for backend '%s'", path.ServicePortString, path.ServicePortInt, endpoints.BackendName)
}

// updateHAProxySrvs dynamically update (via runtime socket) HAProxy backend servers with modified Addresses
func (c *HAProxyController) updateHAProxySrvs(oldEndpoints, newEndpoints *store.Endpoints) {
	if oldEndpoints.BackendName == "" {
		logger.Errorf("No backend available for endpoints of service `%s`", oldEndpoints.Service.Value)
		return
	}
	newEndpoints.HAProxySrvs = oldEndpoints.HAProxySrvs
	newEndpoints.BackendName = oldEndpoints.BackendName
	haproxySrvs := newEndpoints.HAProxySrvs
	newAddresses := newEndpoints.AddrRemain
	usedAddresses := newEndpoints.AddrsUsed
	// Disable stale entries from HAProxySrvs
	// and provide list of Disabled Srvs
	disabledSrvs := make(map[string]struct{})
	for srvName, srv := range haproxySrvs {
		if _, ok := newAddresses[srv.Address]; ok {
			usedAddresses[srv.Address] = struct{}{}
			delete(newAddresses, srv.Address)
		} else {
			haproxySrvs[srvName].Address = ""
			haproxySrvs[srvName].Modified = true
			disabledSrvs[srvName] = struct{}{}
		}
	}
	// Configure new Addresses in available HAProxySrvs
	for newAddr := range newAddresses {
		if len(disabledSrvs) == 0 {
			break
		}
		// Pick a rondom available srv
		for srvName := range disabledSrvs {
			haproxySrvs[srvName].Address = newAddr
			haproxySrvs[srvName].Modified = true
			usedAddresses[newAddr] = struct{}{}
			delete(disabledSrvs, srvName)
			delete(newAddresses, newAddr)
			break
		}
	}
	// Dynamically updates HAProxy backend servers  with HAProxySrvs content
	for srvName, srv := range haproxySrvs {
		if !srv.Modified {
			continue
		}
		if srv.Address == "" {
			logger.Debugf("server '%s/%s' changed status to %v", newEndpoints.BackendName, srvName, "maint")
			logger.Error(c.Client.SetServerAddr(newEndpoints.BackendName, srvName, "127.0.0.1", 0))
			logger.Error(c.Client.SetServerState(newEndpoints.BackendName, srvName, "maint"))
		} else {
			logger.Debugf("server '%s/%s' changed status to %v", newEndpoints.BackendName, srvName, "ready")
			logger.Error(c.Client.SetServerAddr(newEndpoints.BackendName, srvName, srv.Address, 0))
			logger.Error(c.Client.SetServerState(newEndpoints.BackendName, srvName, "ready"))
		}
	}
}
