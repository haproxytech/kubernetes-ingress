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
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

// alignHAproxySrvs adds or removes servers to match server-slots param
func (route *Route) alignHAproxySrvs() {
	haproxySrvs := route.endpoints.HAProxySrvs
	// Get server-slots annotation
	// "servers-increment" is a legacy annotation
	annServerSlots, _ := k8sStore.GetValueFromAnnotations("servers-increment", k8sStore.ConfigMaps[Main].Annotations)
	if annServerSlots == nil {
		annServerSlots, _ = k8sStore.GetValueFromAnnotations("server-slots", k8sStore.ConfigMaps[Main].Annotations)

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
		route.reload = true
	}
	// Remove HAProxySrvs to match required serverSlots
	usedAddrs := route.endpoints.AddrsUsed
	remainAddrs := route.endpoints.AddrRemain
	for len(haproxySrvs) > requiredSrvNbr {
		// pick random server
		for srvName, srv := range haproxySrvs {
			srvAddr := srv.Address
			if _, ok := usedAddrs[srvAddr]; ok {
				delete(usedAddrs, srvAddr)
				remainAddrs[srvAddr] = struct{}{}
			}
			delete(haproxySrvs, srvName)
			logger.Error(client.BackendServerDelete(route.endpoints.BackendName, srvName))
			route.reload = true
			continue
		}
	}
	// Configure remaining addresses in available HAProxySrvs
	for _, srv := range haproxySrvs {
		if srv.Address == "" {
			for addr := range route.endpoints.AddrRemain {
				srv.Address = addr
				srv.Modified = true
				usedAddrs[addr] = struct{}{}
				delete(remainAddrs, addr)
				break
			}
		}
	}
}

// handleEndpoints lookups the IngressPath related endpoints and makes corresponding backend servers configuration in HAProxy
// If only the address changes , no need to reload just generate new config
func (route *Route) handleEndpoints() {
	route.getEndpoints()
	if route.endpoints == nil {
		if client.BackendServerDeleteAll(route.BackendName) {
			route.reload = true
		}
		return
	}
	route.alignHAproxySrvs()
	activeAnnotations := route.getServerAnnotations()
	for srvName, srv := range route.endpoints.HAProxySrvs {
		if srv.Modified || route.NewBackend || activeAnnotations {
			route.handleHAProxSrv(srvName, srv.Address)
		}
	}
}

// handleHAProxSrv creates/updates corresponding HAProxy backend server
func (route *Route) handleHAProxSrv(srvName, srvAddr string) {
	server := models.Server{
		Name:    srvName,
		Address: srvAddr,
		Port:    &route.endpoints.Port,
		Weight:  utils.PtrInt64(128),
	}
	if server.Address == "" {
		server.Address = "127.0.0.1"
		server.Maintenance = "enabled"
	}
	handleServerAnnotations(&server, route.srvAnnotations)
	errAPI := client.BackendServerEdit(route.BackendName, server)
	if errAPI == nil {
		logger.Debugf("Updating server '%s/%s'", route.BackendName, server.Name)
		return
	}
	if strings.Contains(errAPI.Error(), "does not exist") {
		logger.Debugf("Creating server '%s/%s'", route.BackendName, server.Name)
		logger.Error(client.BackendServerCreate(route.BackendName, server))
	}
}

func (route *Route) handleExternalName() {
	//TODO: currently HAProxy will only resolve server name at startup/reload
	// This needs to be improved by using HAProxy resolvers to have resolution at runtime
	logger.Debugf("Configuring service '%s', of type ExternalName", route.service.Name)
	var port int64
	for _, sp := range route.service.Ports {
		if sp.Name == route.Path.ServicePortString || sp.Port == route.Path.ServicePortInt {
			port = sp.Port
		}
	}
	if port == 0 {
		ingressPort := route.Path.ServicePortString
		if route.Path.ServicePortInt != 0 {
			ingressPort = fmt.Sprintf("%d", route.Path.ServicePortInt)
		}
		logger.Warningf("service '%s': service port '%s' not found", route.service.Name, ingressPort)
		return
	}
	route.endpoints = &store.PortEndpoints{
		Port: port,
		HAProxySrvs: map[string]*store.HAProxySrv{
			"external-service": {
				Address:  route.service.DNS,
				Modified: true,
			},
		},
	}
}

func (route *Route) getEndpoints() {
	endpoints, ok := route.Namespace.Endpoints[route.service.Name]
	if !ok {
		if route.service.DNS != "" {
			route.handleExternalName()
		} else {
			logger.Warningf("ingress %s/%s: No Endpoints for service '%s'", route.Namespace.Name, route.Ingress.Name, route.service.Name)
		}
		return
	}
	for _, sp := range route.service.Ports {
		if sp.Name == route.Path.ServicePortString || sp.Port == route.Path.ServicePortInt {
			if endpoints, ok := endpoints.Ports[sp.Name]; ok {
				route.endpoints = endpoints
				return
			}
			logger.Warningf("ingress %s/%s: no matching endpoints for service '%s' and port '%s'", route.Namespace.Name, route.Ingress.Name, route.service.Name, sp.Name)
			return
		}
	}
	ingressPort := route.Path.ServicePortString
	if route.Path.ServicePortInt != 0 {
		ingressPort = fmt.Sprintf("%d", route.Path.ServicePortInt)
	}
	logger.Warningf("ingress %s/%s: service %s: no service port matching '%s'", route.Namespace.Name, route.Ingress.Name, route.service.Name, ingressPort)
}
