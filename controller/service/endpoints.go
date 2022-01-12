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

package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// HandleEndpoints lookups the IngressPath related endpoints and handles corresponding backend servers configuration in HAProxy
func (s *SvcContext) HandleEndpoints(client api.HAProxyClient, store store.K8s, certs *haproxy.Certificates) (reload bool) {
	var srvsScaled bool
	endpoints, err := s.getEndpoints(store)
	if err != nil {
		logger.Warningf("Ingress '%s/%s': %s", s.ingress.Namespace, s.ingress.Name, err)
		return
	}
	// set backendName in store.PortEndpoints for runtime updates.
	endpoints.BackendName = s.backendName
	if s.service.DNS == "" {
		srvsScaled = s.scaleHAProxySrvs(endpoints, store)
	}
	srvsActiveAnn := annotations.HandleServerAnnotations(
		store,
		client,
		certs,
		&models.Server{Namespace: s.service.Name},
		false,
		s.service.Annotations,
		s.ingress.Annotations,
		s.store.ConfigMaps.Main.Annotations,
	)
	if srvsActiveAnn {
		logger.Debugf("Ingress '%s/%s': server options of  backend '%s' were updated, reload required", s.ingress.Namespace, s.ingress.Name, endpoints.BackendName)
	}
	for _, srv := range endpoints.HAProxySrvs {
		if srv.Modified || s.newBackend || srvsActiveAnn {
			server := models.Server{
				Name:    srv.Name,
				Address: srv.Address,
				Port:    &endpoints.Port,
				Weight:  utils.PtrInt64(128),
			}
			s.updateHAProxySrv(server, client, store, certs)
		}
	}

	return srvsScaled || srvsActiveAnn
}

// updateHAProxySrv updates corresponding HAProxy backend server or creates one if it does not exist
func (s *SvcContext) updateHAProxySrv(server models.Server, client api.HAProxyClient, store store.K8s, haproxyCerts *haproxy.Certificates) {
	// Disabled
	if server.Address == "" {
		server.Address = "127.0.0.1"
		server.Maintenance = "enabled"
	}
	// Server related annotations
	annotations.HandleServerAnnotations(
		store,
		client,
		haproxyCerts,
		&server,
		true,
		s.service.Annotations,
		s.ingress.Annotations,
		s.store.ConfigMaps.Main.Annotations,
	)
	// Update server
	errAPI := client.BackendServerEdit(s.backendName, server)
	if errAPI == nil {
		logger.Tracef("Updating server '%s/%s'", s.backendName, server.Name)
		return
	}
	if strings.Contains(errAPI.Error(), "does not exist") {
		logger.Tracef("Creating server '%s/%s'", s.backendName, server.Name)
		logger.Error(client.BackendServerCreate(s.backendName, server))
	}
}

// scaleHAproxySrvs adds servers to match available addresses
func (s *SvcContext) scaleHAProxySrvs(endpoints *store.PortEndpoints, k8sStore store.K8s) (reload bool) {
	var flag bool
	var srvSlots int
	var disabled []*store.HAProxySrv
	// Add disabled HAProxySrvs to match "scale-server-slots"
	// scale-server-slots has a default value in defaultAnnotations
	// "servers-increment", "server-slots" are legacy annotations
	for _, annotation := range []string{"servers-increment", "server-slots", "scale-server-slots"} {
		annServerSlots, _ := k8sStore.GetValueFromAnnotations(annotation, k8sStore.ConfigMaps.Main.Annotations)
		if annServerSlots != nil {
			if value, err := strconv.Atoi(annServerSlots.Value); err == nil {
				srvSlots = value
				break
			} else {
				logger.Error(err)
			}
		}
	}
	for len(endpoints.HAProxySrvs) < srvSlots {
		srv := &store.HAProxySrv{
			Name:     fmt.Sprintf("SRV_%d", len(endpoints.HAProxySrvs)+1),
			Address:  "",
			Modified: true,
		}
		endpoints.HAProxySrvs = append(endpoints.HAProxySrvs, srv)
		disabled = append(disabled, srv)
		flag = true
	}
	if flag {
		reload = true
		logger.Debugf("Server slots in backend '%s' scaled to match scale-server-slots value: %d, reload required", s.backendName, srvSlots)
	}
	// Configure remaining addresses in available HAProxySrvs
	flag = false
	for addr := range endpoints.AddrNew {
		if len(disabled) != 0 {
			disabled[0].Address = addr
			disabled[0].Modified = true
			disabled = disabled[1:]
		} else {
			srv := &store.HAProxySrv{
				Name:     fmt.Sprintf("SRV_%d", len(endpoints.HAProxySrvs)+1),
				Address:  addr,
				Modified: true,
			}
			endpoints.HAProxySrvs = append(endpoints.HAProxySrvs, srv)
			flag = true
		}
		delete(endpoints.AddrNew, addr)
	}
	if flag {
		reload = true
		logger.Debugf("Server slots in backend '%s' scaled to match available endpoints, reload required", s.backendName)
	}
	return reload
}

func (s *SvcContext) getEndpoints(k8s store.K8s) (endpoints *store.PortEndpoints, err error) {
	var ok bool
	var e *store.Endpoints
	if ns := k8s.Namespaces[s.service.Namespace]; ns != nil {
		e, ok = ns.Endpoints[s.service.Name]
	}
	if !ok {
		if s.service.DNS != "" {
			return s.getExternalNameEndpoints()
		}
		return nil, fmt.Errorf("no Endpoints for service '%s'", s.service.Name)
	}
	svcPort := s.path.SvcPortResolved
	if svcPort != nil && e.Ports[svcPort.Name] != nil {
		return e.Ports[svcPort.Name], nil
	}
	if s.path.SvcPortString != "" {
		return nil, fmt.Errorf("no matching endpoints for service '%s' and port '%s'", s.service.Name, s.path.SvcPortString)
	}
	return nil, fmt.Errorf("no matching endpoints for service '%s' and port '%d'", s.service.Name, s.path.SvcPortInt)
}

func (s *SvcContext) getExternalNameEndpoints() (endpoints *store.PortEndpoints, err error) {
	logger.Tracef("Configuring service '%s', of type ExternalName", s.service.Name)
	var port int64
	for _, sp := range s.service.Ports {
		if sp.Name == s.path.SvcPortString || sp.Port == s.path.SvcPortInt {
			port = sp.Port
		}
	}
	if port == 0 {
		ingressPort := s.path.SvcPortString
		if s.path.SvcPortInt != 0 {
			ingressPort = fmt.Sprintf("%d", s.path.SvcPortInt)
		}
		return nil, fmt.Errorf("service '%s': service port '%s' not found", s.service.Name, ingressPort)
	}
	endpoints = &store.PortEndpoints{
		Port: port,
		HAProxySrvs: []*store.HAProxySrv{
			{
				Name:     "SRV_1",
				Address:  s.service.DNS,
				Modified: true,
			},
		},
	}
	return endpoints, nil
}
