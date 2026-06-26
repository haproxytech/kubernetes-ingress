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
	"errors"
	"fmt"
	"strconv"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// externalNameServer is the one server of a DNS backend; renames edit it in place.
const externalNameServer = "SRV_1"

// HandleHAProxySrvs handles the haproxy backend servers of the corresponding IngressPath (service + port)
func (s *Service) HandleHAProxySrvs(k8s store.K8s, client api.HAProxyClient) {
	backend, err := s.getRuntimeBackend(k8s)
	if err != nil {
		if s.backend != nil && s.backend.Name == store.DefaultLocalBackend {
			return
		}
		logger.Warningf("Ingress '%s/%s': %s", s.resource.Namespace, s.resource.Name, err)
		if servers, _ := client.BackendServersGet(s.backend.Name); servers != nil {
			_ = client.BackendServerDeleteAll(s.backend.Name)
		}
		return
	}
	backend.Name = s.backend.Name // set backendName in store.PortEndpoints for runtime updates.
	for _, name := range []string{"scale-server-slots", "server-slots", "servers-increment"} {
		if annotations.String(name, s.annotations...) != "" {
			logger.Warningf("backend '%s': annotation [%s] is DEPRECATED and ignored, servers are added through the runtime API", s.backend.Name, name)
		}
	}
	// scale servers
	if s.resource.DNS == "" {
		s.scaleHAProxySrvs(backend)
	} else if current, _ := client.BackendServerGet(externalNameServer, s.backend.Name); current != nil {
		// The runtime never learns about a hostname edit; only a reload does.
		srv := backend.HAProxySrvs[externalNameServer]
		changed := current.Address != srv.Address || current.Port == nil || *current.Port != srv.Port
		instance.ReloadIf(changed, "backend '%s': external name target changed", s.backend.Name)
	}
	// update servers
	for _, srvSlot := range backend.HAProxySrvs {
		if srvSlot.Modified || s.newBackend || s.serversToEdit {
			s.updateHAProxySrv(client, *srvSlot)
		}
	}
	// Deleted servers were just written as MAINT; the final commit drops them from the file.
	for _, server := range backend.HAProxySrvs {
		if server.Deleted {
			delete(backend.HAProxySrvs, server.Name)
		}
	}

	if backend.DynUpdateFailed {
		backend.DynUpdateFailed = false
		instance.Reload("backend '%s': dynamic update failed", backend.Name)
	}
}

// scaleHAproxySrvs adds servers to match available addresses
func (s *Service) scaleHAProxySrvs(backend *store.RuntimeBackend) {
	if backend.HAProxySrvs == nil {
		backend.HAProxySrvs = make(map[string]*store.HAProxySrv)
	}
	for _, runtimeEndpoint := range store.SortedRuntimeEndpoints(backend.Endpoints) {
		srv := &store.HAProxySrv{
			Name:     runtimeEndpoint.ComputeServerName(),
			Address:  runtimeEndpoint.Address,
			Port:     runtimeEndpoint.Port,
			Modified: true,
		}
		backend.HAProxySrvs[srv.Name] = srv
	}
	backend.Endpoints = map[store.RuntimeEndpoint]struct{}{}
}

// updateHAProxySrv updates corresponding HAProxy backend server or creates one if it does not exist
func (s *Service) updateHAProxySrv(client api.HAProxyClient, srvSlot store.HAProxySrv) {
	srv := models.Server{
		Name:         srvSlot.Name,
		Port:         utils.PtrInt64(1),
		Address:      "127.0.0.1",
		ServerParams: models.ServerParams{Maintenance: "enabled"},
	}
	if s.backend.Cookie != nil && !s.backend.Cookie.Dynamic {
		srv.ServerParams.Cookie = srvSlot.Name
	}
	// Enable Server
	if srvSlot.Address != "" {
		srv.Address = srvSlot.Address
		srv.Port = utils.PtrInt64(srvSlot.Port)
		srv.Maintenance = "disabled"
	}

	//revive:disable-next-line:line-length-limit
	logger.Debugf("[CONFIG] [BACKEND] [SERVER] [UPDATE] backend %s: about to update server in configuration file :  models.Server { Name: %s, Port: %d, Address: %s, Maintenance: %s }", s.backend.Name, srv.Name, *srv.Port, srv.Address, srv.Maintenance)
	errAPI := client.BackendServerCreateOrUpdate(s.backend.Name, srv)
	if errAPI != nil {
		logger.Errorf("[CONFIG] [BACKEND] [SERVER] %v", errAPI)
	}
}

func (s *Service) getRuntimeBackend(k8s store.K8s) (backend *store.RuntimeBackend, err error) {
	if s.resource.DNS != "" {
		return s.getExternalNameEndpoints()
	}
	var ok bool
	var backends map[string]*store.RuntimeBackend
	if ns := k8s.Namespaces[s.resource.Namespace]; ns != nil {
		backends, ok = ns.HAProxyRuntime[s.resource.Name]
	}
	if !ok {
		return nil, errors.New("no available endpoints")
	}
	svcPort := s.path.SvcPortResolved
	if svcPort != nil && backends[svcPort.Name] != nil {
		return backends[svcPort.Name], nil
	}
	if s.path.SvcPortString != "" {
		return nil, fmt.Errorf("no matching endpoints for port '%s'", s.path.SvcPortString)
	}
	return nil, fmt.Errorf("no matching endpoints for port '%d'", s.path.SvcPortInt)
}

func (s *Service) getExternalNameEndpoints() (endpoints *store.RuntimeBackend, err error) {
	logger.Tracef("Configuring service '%s', of type ExternalName", s.resource.Name)
	var port int64
	for _, sp := range s.resource.Ports {
		if sp.Name == s.path.SvcPortString || sp.Port == s.path.SvcPortInt {
			port = sp.Port
		}
	}
	if port == 0 {
		ingressPort := s.path.SvcPortString
		if s.path.SvcPortInt != 0 {
			ingressPort = strconv.FormatInt(s.path.SvcPortInt, 10)
		}
		return nil, fmt.Errorf("service '%s': service port '%s' not found", s.resource.Name, ingressPort)
	}
	endpoints = &store.RuntimeBackend{
		HAProxySrvs: map[string]*store.HAProxySrv{
			externalNameServer: {
				Name:     externalNameServer,
				Address:  s.resource.DNS,
				Port:     port,
				Modified: true,
			},
		},
	}
	return endpoints, nil
}
