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

// scaleHAproxySrvs adds servers to match available addresses
func (route *Route) scaleHAProxySrvs() {
	var srvSlots int
	var disabled []*store.HAProxySrv
	haproxySrvs := route.endpoints.HAProxySrvs
	// "servers-increment", "server-slots" are legacy annotations
	for _, annotation := range []string{"servers-increment", "server-slots", "scale-server-slots"} {
		annServerSlots, _ := k8sStore.GetValueFromAnnotations(annotation, k8sStore.ConfigMaps[Main].Annotations)
		if annServerSlots != nil {
			if value, err := strconv.Atoi(annServerSlots.Value); err == nil {
				srvSlots = value
				break
			} else {
				logger.Error(err)
			}
		}
	}
	// Add disabled HAProxySrvs to match scale-server-slots
	for len(haproxySrvs) < srvSlots {
		srv := &store.HAProxySrv{
			Name:     fmt.Sprintf("SRV_%d", len(haproxySrvs)+1),
			Address:  "",
			Modified: true,
		}
		haproxySrvs = append(haproxySrvs, srv)
		disabled = append(disabled, srv)
		route.reload = true
	}
	// Configure remaining addresses in available HAProxySrvs
	for addr := range route.endpoints.AddrNew {
		if len(disabled) != 0 {
			disabled[0].Address = addr
			disabled[0].Modified = true
			disabled = disabled[1:]
		} else {
			srv := &store.HAProxySrv{
				Name:     fmt.Sprintf("SRV_%d", len(haproxySrvs)+1),
				Address:  addr,
				Modified: true,
			}
			haproxySrvs = append(haproxySrvs, srv)
			route.reload = true
		}
		delete(route.endpoints.AddrNew, addr)
	}
	route.endpoints.HAProxySrvs = haproxySrvs
}

// handleEndpoints lookups the IngressPath related endpoints and makes corresponding backend servers configuration in HAProxy
// If only the address changes , no need to reload just generate new config
func (route *Route) handleEndpoints() {
	route.getEndpoints()
	if route.endpoints == nil {
		return
	}
	route.endpoints.BackendName = route.BackendName
	if route.service.DNS == "" {
		route.scaleHAProxySrvs()
	}
	backendUpdated := route.NewBackend
	backendUpdated = route.getSrvAnnotations() || backendUpdated
	backendUpdated = route.handleSrvSSLAnnotations() || backendUpdated
	route.reload = route.reload || backendUpdated
	for _, srv := range route.endpoints.HAProxySrvs {
		if srv.Modified || backendUpdated {
			route.handleHAProxSrv(srv)
		}
	}
}

// handleHAProxSrv creates/updates corresponding HAProxy backend server
func (route *Route) handleHAProxSrv(srv *store.HAProxySrv) {
	server := models.Server{
		Name:    srv.Name,
		Address: srv.Address,
		Port:    &route.endpoints.Port,
		Weight:  utils.PtrInt64(128),
	}
	if server.Address == "" {
		server.Address = "127.0.0.1"
		server.Maintenance = "enabled"
	}
	if route.sslServer.enabled {
		server.Ssl = "enabled"
		server.Alpn = "h2,http/1.1"
		server.Verify = "none"
	}
	handleSrvAnnotations(&server, route.srvAnnotations)
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
		HAProxySrvs: []*store.HAProxySrv{
			{
				Name:     "SRV_1",
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
