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

func (c *HAProxyController) getSrvSlotsAnn() (min, max int) {
	// "servers-increment", "server-slots" are legacy annotations
	for _, annotation := range []string{"servers-increment", "server-slots", "min-server-slots"} {
		annMinServerSlots, _ := GetValueFromAnnotations(annotation, c.cfg.ConfigMap.Annotations)
		if annMinServerSlots == nil {
			continue
		}
		if value, err := strconv.Atoi(annMinServerSlots.Value); err == nil {
			min = value
			break
		} else {
			c.Logger.Error(err)
		}
	}
	annMaxServerSlots, _ := GetValueFromAnnotations("max-server-slots", c.cfg.ConfigMap.Annotations)
	if annMaxServerSlots != nil {
		if value, err := strconv.Atoi(annMaxServerSlots.Value); err == nil {
			max = value
		} else {
			c.Logger.Error(err)
		}
	}
	if max != 0 && max < min {
		c.Logger.Errorf("max-server-slots value %d is less than min-server-slots value %d", max, min)
		return min, 0
	}
	return min, max
}

// alignHAproxySrvs adds or removes servers to match server-slots param
func (c *HAProxyController) alignHAproxySrvs(endpoints *PortEndpoints) (reload bool) {
	haproxySrvs := endpoints.HAProxySrvs
	minSrvNbr, maxSrvNbr := c.getSrvSlotsAnn()
	disabled := []string{}
	// Add disabled HAProxySrvs to match min-server-slots
	for len(haproxySrvs) < minSrvNbr {
		srvName := fmt.Sprintf("SRV_%s", utils.RandomString(5))
		haproxySrvs[srvName] = &HAProxySrv{
			Address:  "",
			Modified: true,
		}
		disabled = append(disabled, srvName)
		reload = true
	}
	// When max-server-slots enabled (> 0):
	// Remove HAProxySrvs to match maximum serverSlots
	for srvName, srv := range haproxySrvs {
		if maxSrvNbr == 0 || maxSrvNbr >= len(haproxySrvs) {
			break
		}
		endpoints.AddrRemain[srv.Address] = struct{}{}
		delete(haproxySrvs, srvName)
		c.Logger.Error(c.Client.BackendServerDelete(endpoints.BackendName, srvName))
		reload = true
	}
	// Configure remaining addresses in available HAProxySrvs
	// when max-server-slots enabled (>0)
	for addr := range endpoints.AddrRemain {
		switch {
		case len(disabled) != 0:
			srv := haproxySrvs[disabled[0]]
			srv.Address = addr
			srv.Modified = true
			disabled = disabled[1:]
			delete(endpoints.AddrRemain, addr)
		case maxSrvNbr == 0 || maxSrvNbr > len(haproxySrvs):
			srvName := fmt.Sprintf("SRV_%s", utils.RandomString(5))
			haproxySrvs[srvName] = &HAProxySrv{
				Address:  addr,
				Modified: true,
			}
			delete(endpoints.AddrRemain, addr)
			reload = true
		default:
			break
		}
	}
	return reload
}

// handleEndpoints lookups the IngressPath related endpoints and makes corresponding backend servers configuration in HAProxy
// If only the address changes , no need to reload just generate new config
func (c *HAProxyController) handleEndpoints(namespace *Namespace, ingress *Ingress, path *IngressPath, service *Service, backendName string, newBackend bool) (reload bool) {
	reload = newBackend
	endpoints := c.getEndpoints(namespace, ingress, path, service)
	if endpoints == nil {
		return reload
	}
	// Handle Backend servers
	endpoints.BackendName = backendName
	annotations, activeAnnotations := c.getServerAnnotations(ingress, service)
	if service.DNS == "" {
		reload = c.alignHAproxySrvs(endpoints) || reload
	}
	reload = reload || activeAnnotations
	for srvName, srv := range endpoints.HAProxySrvs {
		if srv.Modified || reload {
			c.handleHAProxSrv(srvName, srv.Address, backendName, endpoints.Port, annotations)
		}
	}
	return reload
}

// handleHAProxSrv creates/updates corresponding HAProxy backend server
func (c *HAProxyController) handleHAProxSrv(srvName, srvAddr, backendName string, port int64, annotations map[string]*StringW) {
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
		c.Logger.Debugf("Updating server '%s/%s'", backendName, server.Name)
		return
	}
	if strings.Contains(errAPI.Error(), "does not exist") {
		c.Logger.Debugf("Creating server '%s/%s'", backendName, server.Name)
		c.Logger.Error(c.Client.BackendServerCreate(backendName, server))
	}
}

func (c *HAProxyController) handleExternalName(path *IngressPath, service *Service) *PortEndpoints {
	//TODO: currently HAProxy will only resolve server name at startup/reload
	// This needs to be improved by using HAProxy resolvers to have resolution at runtime
	c.Logger.Debugf("Configuring service '%s', of type ExternalName", service.Name)
	var port int64
	for _, sp := range service.Ports {
		if sp.Name == path.ServicePortString || sp.Port == path.ServicePortInt {
			port = sp.Port
		}
	}
	if port == 0 {
		ingressPort := path.ServicePortString
		if path.ServicePortInt != 0 {
			ingressPort = fmt.Sprintf("%d", path.ServicePortInt)
		}
		c.Logger.Warningf("service '%s': service port '%s' not found", service.Name, ingressPort)
		return nil
	}
	return &PortEndpoints{
		Port: port,
		HAProxySrvs: map[string]*HAProxySrv{
			"external-service": {
				Address:  service.DNS,
				Modified: true,
			},
		},
	}
}

func (c *HAProxyController) getEndpoints(namespace *Namespace, ingress *Ingress, path *IngressPath, service *Service) *PortEndpoints {
	endpoints, ok := namespace.Endpoints[service.Name]
	if !ok {
		if service.DNS == "" {
			c.Logger.Warningf("No Endpoints for service '%s'", service.Name)
			return nil
		}
		return c.handleExternalName(path, service)
	}
	for _, sp := range service.Ports {
		if sp.Name == path.ServicePortString || sp.Port == path.ServicePortInt {
			if endpoints, ok := endpoints.Ports[sp.Name]; ok {
				return endpoints
			}
			c.Logger.Warningf("ingress %s/%s: no matching endpoints for service '%s' and port '%s'", namespace.Name, ingress.Name, service.Name, sp.Name)

			return nil
		}
	}
	ingressPort := path.ServicePortString
	if path.ServicePortInt != 0 {
		ingressPort = fmt.Sprintf("%d", path.ServicePortInt)
	}
	c.Logger.Warningf("ingress %s/%s: service %s: no service port matching '%s'", namespace.Name, ingress.Name, service.Name, ingressPort)
	return nil
}

// updateHAProxySrvs dynamically update (via runtime socket) HAProxy backend servers with modifged Addresses
func (c *HAProxyController) updateHAProxySrvs(oldEndpoints, newEndpoints *PortEndpoints) {
	if oldEndpoints.BackendName == "" {
		return
	}
	newEndpoints.HAProxySrvs = oldEndpoints.HAProxySrvs
	newEndpoints.BackendName = oldEndpoints.BackendName
	haproxySrvs := newEndpoints.HAProxySrvs
	newAddresses := newEndpoints.AddrRemain
	// Disable stale entries from HAProxySrvs
	// and provide list of Disabled Srvs
	disabledSrvs := make(map[string]struct{})
	for srvName, srv := range haproxySrvs {
		if _, ok := newAddresses[srv.Address]; ok {
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
			c.Logger.Debugf("server '%s/%s' changed status to %v", newEndpoints.BackendName, srvName, "maint")
			c.Logger.Error(c.Client.SetServerAddr(newEndpoints.BackendName, srvName, "127.0.0.1", 0))
			c.Logger.Error(c.Client.SetServerState(newEndpoints.BackendName, srvName, "maint"))
		} else {
			c.Logger.Debugf("server '%s/%s' changed status to %v", newEndpoints.BackendName, srvName, "ready")
			c.Logger.Error(c.Client.SetServerAddr(newEndpoints.BackendName, srvName, srv.Address, 0))
			c.Logger.Error(c.Client.SetServerState(newEndpoints.BackendName, srvName, "ready"))
		}
	}
}
