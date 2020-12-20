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

// scaleHAproxySrvs adds servers to match available addresses
func (c *HAProxyController) scaleHAproxySrvs(endpoints *PortEndpoints) (reload bool) {
	var srvSlots int
	var disabled []*HAProxySrv
	haproxySrvs := endpoints.HAProxySrvs
	// "servers-increment", "server-slots" are legacy annotations
	for _, annotation := range []string{"servers-increment", "server-slots", "scale-server-slots"} {
		annServerSlots, _ := GetValueFromAnnotations(annotation, c.cfg.ConfigMap.Annotations)
		if annServerSlots != nil {
			if value, err := strconv.Atoi(annServerSlots.Value); err == nil {
				srvSlots = value
				break
			} else {
				c.Logger.Error(err)
			}
		}
	}
	// Add disabled HAProxySrvs to match scale-server-slots
	for len(haproxySrvs) < srvSlots {
		srv := &HAProxySrv{
			Name:     fmt.Sprintf("SRV_%s", utils.RandomString(5)),
			Address:  "",
			Modified: true,
		}
		haproxySrvs = append(haproxySrvs, srv)
		disabled = append(disabled, srv)
		reload = true
	}
	// Configure remaining addresses in available HAProxySrvs
	for addr := range endpoints.AddrNew {
		if len(disabled) != 0 {
			disabled[0].Address = addr
			disabled[0].Modified = true
			disabled = disabled[1:]
		} else {
			srv := &HAProxySrv{
				Name:     fmt.Sprintf("SRV_%s", utils.RandomString(5)),
				Address:  addr,
				Modified: true,
			}
			haproxySrvs = append(haproxySrvs, srv)
			reload = true
		}
		delete(endpoints.AddrNew, addr)
	}
	endpoints.HAProxySrvs = haproxySrvs
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
		reload = c.scaleHAproxySrvs(endpoints) || reload
	}
	reload = reload || activeAnnotations
	for _, srv := range endpoints.HAProxySrvs {
		if srv.Modified || reload {
			c.handleHAProxSrv(srv, backendName, endpoints.Port, annotations)
		}
	}
	return reload
}

// handleHAProxSrv creates/updates corresponding HAProxy backend server
func (c *HAProxyController) handleHAProxSrv(srv *HAProxySrv, backendName string, port int64, annotations map[string]*StringW) {
	server := models.Server{
		Name:    srv.Name,
		Address: srv.Address,
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
		HAProxySrvs: []*HAProxySrv{{
			Name:     "SRV_1",
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

// updateHAProxySrvs dynamically updates (via runtime socket) HAProxy backend servers with modifged Addresses
func (c *HAProxyController) updateHAProxySrvs(oldEndpoints, newEndpoints *PortEndpoints) {
	if oldEndpoints.BackendName == "" {
		return
	}
	newEndpoints.HAProxySrvs = oldEndpoints.HAProxySrvs
	newEndpoints.BackendName = oldEndpoints.BackendName
	haproxySrvs := newEndpoints.HAProxySrvs
	newAddresses := newEndpoints.AddrNew
	// Disable stale entries from HAProxySrvs
	// and provide list of Disabled Srvs
	var disabled []*HAProxySrv
	for i, srv := range haproxySrvs {
		if _, ok := newAddresses[srv.Address]; ok {
			delete(newAddresses, srv.Address)
		} else {
			haproxySrvs[i].Address = ""
			haproxySrvs[i].Modified = true
			disabled = append(disabled, srv)
		}
	}
	// Configure new Addresses in available HAProxySrvs
	for newAddr := range newAddresses {
		if len(disabled) == 0 {
			break
		}
		disabled[0].Address = newAddr
		disabled[0].Modified = true
		disabled = disabled[1:]
		delete(newAddresses, newAddr)
	}
	// Dynamically updates HAProxy backend servers  with HAProxySrvs content
	for _, srv := range haproxySrvs {
		if !srv.Modified {
			continue
		}
		if srv.Address == "" {
			c.Logger.Debugf("server '%s/%s' changed status to %v", newEndpoints.BackendName, srv.Name, "maint")
			c.Logger.Error(c.Client.SetServerAddr(newEndpoints.BackendName, srv.Name, "127.0.0.1", 0))
			c.Logger.Error(c.Client.SetServerState(newEndpoints.BackendName, srv.Name, "maint"))
		} else {
			c.Logger.Debugf("server '%s/%s' changed status to %v", newEndpoints.BackendName, srv.Name, "ready")
			c.Logger.Error(c.Client.SetServerAddr(newEndpoints.BackendName, srv.Name, srv.Address, 0))
			c.Logger.Error(c.Client.SetServerState(newEndpoints.BackendName, srv.Name, "ready"))
		}
	}
}
