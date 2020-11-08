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
func (c *HAProxyController) alignHAproxySrvs(endpoints *store.PortEndpoints) (reload bool) {
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
func (c *HAProxyController) handleEndpoints(route *IngressRoute) (reload bool) {
	endpoints := c.getEndpoints(route)
	if endpoints == nil {
		if c.Client.BackendServerDeleteAll(route.BackendName) {
			reload = true
		}
		return reload
	}
	// Handle Backend servers
	endpoints.BackendName = route.BackendName
	annotations, activeAnnotations := c.getServerAnnotations(route.Ingress, route.Service)
	srvsNbrChanged := c.alignHAproxySrvs(endpoints)
	reload = reload || srvsNbrChanged || activeAnnotations
	for srvName, srv := range endpoints.HAProxySrvs {
		if srv.Modified || route.NewBackend {
			c.handleHAProxSrv(srvName, srv.Address, route.BackendName, endpoints.Port, annotations)
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

func (c *HAProxyController) handleExternalName(route *IngressRoute) *store.PortEndpoints {
	//TODO: currently HAProxy will only resolve server name at startup/reload
	// This needs to be improved by using HAProxy resolvers to have resolution at runtime
	logger.Debugf("Configuring service '%s', of type ExternalName", route.Service.Name)
	var port int64
	for _, sp := range route.Service.Ports {
		if sp.Name == route.Path.ServicePortString || sp.Port == route.Path.ServicePortInt {
			port = sp.Port
		}
	}
	if port == 0 {
		ingressPort := route.Path.ServicePortString
		if route.Path.ServicePortInt != 0 {
			ingressPort = fmt.Sprintf("%d", route.Path.ServicePortInt)
		}
		logger.Warningf("service '%s': service port '%s' not found", route.Service.Name, ingressPort)
		return nil
	}
	return &store.PortEndpoints{
		Port: port,
		HAProxySrvs: map[string]*store.HAProxySrv{
			"external-service": {
				Address:  route.Service.DNS,
				Modified: true,
			},
		},
	}
}

func (c *HAProxyController) getEndpoints(route *IngressRoute) *store.PortEndpoints {
	endpoints, ok := route.Namespace.Endpoints[route.Service.Name]
	if !ok {
		if route.Service.DNS == "" {
			logger.Warningf("ingress %s/%s: No Endpoints for service '%s'", route.Namespace.Name, route.Ingress.Name, route.Service.Name)
			return nil
		}
		return c.handleExternalName(route)
	}
	for _, sp := range route.Service.Ports {
		if sp.Name == route.Path.ServicePortString || sp.Port == route.Path.ServicePortInt {
			if endpoints, ok := endpoints.Ports[sp.Name]; ok {
				return endpoints
			}
			logger.Warningf("ingress %s/%s: no matching endpoints for service '%s' and port '%s'", route.Namespace.Name, route.Ingress.Name, route.Service.Name, sp.Name)

			return nil
		}
	}
	ingressPort := route.Path.ServicePortString
	if route.Path.ServicePortInt != 0 {
		ingressPort = fmt.Sprintf("%d", route.Path.ServicePortInt)
	}
	logger.Warningf("ingress %s/%s: service %s: no service port matching '%s'", route.Namespace.Name, route.Ingress.Name, route.Service.Name, ingressPort)
	return nil
}

// updateHAProxySrvs dynamically update (via runtime socket) HAProxy backend servers with modifged Addresses
func (c *HAProxyController) updateHAProxySrvs(oldEndpoints, newEndpoints *store.PortEndpoints) {
	if oldEndpoints.BackendName == "" {
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
