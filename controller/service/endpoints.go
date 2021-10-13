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
	"strings"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

// HandleHAProxySrvs handles the haproxy backend servers of the corresponding IngressPath (service + port)
func (s *SvcContext) HandleHAProxySrvs(client api.HAProxyClient, store store.K8s) (reload bool) {
	var srvsScaled bool
	backend, err := s.getRuntimeBackend(store)
	if err != nil {
		logger.Warningf("Ingress '%s/%s': %s", s.ingress.Namespace, s.ingress.Name, err)
		if servers, _ := client.BackendServersGet(s.backend.Name); servers != nil {
			client.BackendServerDeleteAll(s.backend.Name)
		}
		return
	}
	backend.Name = s.backend.Name // set backendName in store.PortEndpoints for runtime updates.
	// scale servers
	if s.service.DNS == "" {
		srvsScaled = s.scaleHAProxySrvs(backend, store)
	}
	// update servers
	for _, srvSlot := range backend.HAProxySrvs {
		if srvSlot.Modified || s.newBackend {
			s.updateHAProxySrv(client, *srvSlot, backend.Endpoints.Port)
		}
	}
	if backend.DynUpdateFailed {
		backend.DynUpdateFailed = false
		return true
	}
	return srvsScaled
}

// updateHAProxySrv updates corresponding HAProxy backend server or creates one if it does not exist
func (s *SvcContext) updateHAProxySrv(client api.HAProxyClient, srvSlot store.HAProxySrv, port int64) {
	srv := models.Server{
		Name:        srvSlot.Name,
		Port:        &port,
		Address:     "127.0.0.1",
		Maintenance: "enabled",
	}
	// Enable Server
	if srvSlot.Address != "" {
		srv.Address = srvSlot.Address
		srv.Maintenance = "disabled"
	}
	// Cookie/Session persistence
	if s.backend.Cookie != nil && s.backend.Cookie.Type == "insert" {
		srv.Cookie = srv.Name
	}
	// Update server
	errAPI := client.BackendServerEdit(s.backend.Name, srv)
	if errAPI == nil {
		logger.Tracef("Updating server '%s/%s'", s.backend.Name, srv.Name)
		return
	}
	// Create server
	if strings.Contains(errAPI.Error(), "does not exist") {
		logger.Tracef("Creating server '%s/%s'", s.backend.Name, srv.Name)
		logger.Error(client.BackendServerCreate(s.backend.Name, srv))
	}
}

// scaleHAproxySrvs adds servers to match available addresses
func (s *SvcContext) scaleHAProxySrvs(backend *store.RuntimeBackend, k8sStore store.K8s) (reload bool) {
	var flag bool
	var disabled []*store.HAProxySrv
	var annVal int
	var annErr error
	// Add disabled HAProxySrvs to match "scale-server-slots"
	// scale-server-slots has a default value in defaultAnnotations
	// "servers-increment", "server-slots" are legacy annotations
	srvSlots := 42
	for _, annotation := range []string{"servers-increment", "server-slots", "scale-server-slots"} {
		annVal, annErr = annotations.Int(annotation, k8sStore.ConfigMaps.Main.Annotations)
		if annErr != nil {
			logger.Errorf("Scale HAProxy servers: %s", annErr)
		} else if annVal != 0 {
			srvSlots = annVal
			break
		}
	}
	for len(backend.HAProxySrvs) < srvSlots {
		srv := &store.HAProxySrv{
			Name:     fmt.Sprintf("SRV_%d", len(backend.HAProxySrvs)+1),
			Address:  "",
			Modified: true,
		}
		backend.HAProxySrvs = append(backend.HAProxySrvs, srv)
		disabled = append(disabled, srv)
		flag = true
	}
	if flag {
		reload = true
		logger.Debugf("Server slots in backend '%s' scaled to match scale-server-slots value: %d, reload required", s.backend.Name, srvSlots)
	}
	// Configure remaining addresses in available HAProxySrvs
	flag = false
	for addr := range backend.Endpoints.Addresses {
		if len(disabled) != 0 {
			disabled[0].Address = addr
			disabled[0].Modified = true
			disabled = disabled[1:]
		} else {
			srv := &store.HAProxySrv{
				Name:     fmt.Sprintf("SRV_%d", len(backend.HAProxySrvs)+1),
				Address:  addr,
				Modified: true,
			}
			backend.HAProxySrvs = append(backend.HAProxySrvs, srv)
			flag = true
		}
		delete(backend.Endpoints.Addresses, addr)
	}
	if flag {
		reload = true
		logger.Debugf("Server slots in backend '%s' scaled to match available endpoints, reload required", s.backend.Name)
	}
	return reload
}

func (s *SvcContext) getRuntimeBackend(k8s store.K8s) (backend *store.RuntimeBackend, err error) {
	var ok bool
	var backends map[string]*store.RuntimeBackend
	if ns := k8s.Namespaces[s.service.Namespace]; ns != nil {
		backends, ok = ns.HAProxyRuntime[s.service.Name]
	}
	if !ok {
		if s.service.DNS != "" {
			return s.getExternalNameEndpoints()
		}
		return nil, fmt.Errorf("no Endpoints for service '%s'", s.service.Name)
	}
	sp := s.path.SvcPortResolved
	if sp != nil {
		for portName, backend := range backends {
			if portName == sp.Name || backend.Endpoints.Port == sp.Port {
				return backend, nil
			}
		}
	}
	if s.path.SvcPortString != "" {
		return nil, fmt.Errorf("no matching endpoints for service '%s' and port '%s'", s.service.Name, s.path.SvcPortString)
	}
	return nil, fmt.Errorf("no matching endpoints for service '%s' and port '%d'", s.service.Name, s.path.SvcPortInt)
}

func (s *SvcContext) getExternalNameEndpoints() (endpoints *store.RuntimeBackend, err error) {
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
	endpoints = &store.RuntimeBackend{
		Endpoints: store.PortEndpoints{Port: port},
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
