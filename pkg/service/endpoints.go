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
	// scale servers
	if s.resource.DNS == "" {
		s.scaleHAProxySrvs(backend)
	}
	// update servers
	for _, srvSlot := range backend.HAProxySrvs {
		if srvSlot.Modified || s.newBackend || s.serversToEdit {
			s.updateHAProxySrv(client, *srvSlot)
		}
	}
	if backend.DynUpdateFailed {
		backend.DynUpdateFailed = false
		instance.Reload("backend '%s': dynamic update failed", backend.Name)
	}
}

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
	logger.Tracef("[CONFIG] [BACKEND] [SERVER] backend %s: about to update server in configuration file :  models.Server { Name: %s, Port: %d, Address: %s, Maintenance: %s }", s.backend.Name, srv.Name, *srv.Port, srv.Address, srv.Maintenance)

	errAPI := client.BackendServerCreateOrUpdate(s.backend.Name, srv)
	if errAPI == nil {
		logger.Tracef("[CONFIG] [BACKEND] [SERVER] Creating/Updating server '%s/%s'", s.backend.Name, srv.Name)
	}
}

// scaleHAproxySrvs adds servers to match available addresses
func (s *Service) scaleHAProxySrvs(backend *store.RuntimeBackend) {
	var annVal int
	var annErr error
	// Add disabled HAProxySrvs to match "scale-server-slots"
	// scale-server-slots has a default value in defaultAnnotations
	// "servers-increment", "server-slots" are legacy annotations
	srvSlots := 42
	for _, annotation := range []string{"servers-increment", "server-slots", "scale-server-slots"} {
		annVal, annErr = annotations.Int(annotation, s.annotations...)
		if annErr != nil {
			logger.Errorf("[CONFIG] [BACKEND] [SERVER] Scale HAProxy servers: %s", annErr)
		} else if annVal != 0 {
			srvSlots = annVal
			break
		}
	}
	// We expect to have these slots : the already existing ones from backend.HAProxySrvs and the new ones to be added backend.Endpoints
	// Keep in mind this is about slots not servers. New servers can be already added to backend.HAProxySrvs if the room is sufficient.
	// The name backend.Endpoints is misleading, it's really about new slots that are parts of new servers and can't have been added directly.
	expectedSrvSlots := len(backend.Endpoints) + len(backend.HAProxySrvs)
	// We want at least the expected number of slots ...
	newSrvSlots := expectedSrvSlots
	// ... but if it's not a modulo srvSlots or if it's zero (shouldn't happen) ...
	if expectedSrvSlots%srvSlots != 0 || expectedSrvSlots == 0 {
		// ... we compute the nearest number of slots greather than expectedSrvSlots and being a modulo of srvSlots
		newSrvSlots = expectedSrvSlots - (expectedSrvSlots % srvSlots) + srvSlots
	}

	// Get the number of enabled servers in the current list of servers.
	enabledSlots := 0
	for _, server := range backend.HAProxySrvs {
		if server.Address != "" {
			enabledSlots++
		}
	}
	// If we have to add new slots we'll have to reload, so we can expand the number of free slots by the number srvSlots.
	// But we should add any only if there is no room left in the existing list of servers.
	if enabledSlots+len(backend.Endpoints) > len(backend.HAProxySrvs) &&
		newSrvSlots-(enabledSlots+len(backend.Endpoints)) < srvSlots && newSrvSlots > srvSlots {
		newSrvSlots += srvSlots
	}

	// Create the future slice of slots of the size newSrvSlots ...
	slots := make([]*store.HAProxySrv, newSrvSlots)
	// ... copy the existing servers into ...
	copy(slots, backend.HAProxySrvs)
	i := len(backend.HAProxySrvs)
	// ... then add the new slots ...
	for endpoint := range backend.Endpoints {
		srv := &store.HAProxySrv{
			Name:     fmt.Sprintf("SRV_%d", i+1),
			Address:  endpoint.Address,
			Port:     endpoint.Port,
			Modified: true,
		}
		slots[i] = srv
		i++
	}
	// ... fill in the remaining slots with disabled (empty address) slots.
	for j := i; j < len(slots); j++ {
		srv := &store.HAProxySrv{
			Name:     fmt.Sprintf("SRV_%d", j+1),
			Address:  "",
			Port:     1,
			Modified: true,
		}
		slots[j] = srv
	}
	instance.ReloadIf(len(backend.HAProxySrvs) < len(slots), "[CONFIG] [BACKEND] [SERVER] Server slots in backend '%s' scaled to match available endpoints", s.backend.Name)
	backend.Endpoints = store.RuntimeEndpoints{}
	backend.HAProxySrvs = slots
}

func (s *Service) getRuntimeBackend(k8s store.K8s) (backend *store.RuntimeBackend, err error) {
	var ok bool
	var backends map[string]*store.RuntimeBackend
	if ns := k8s.Namespaces[s.resource.Namespace]; ns != nil {
		backends, ok = ns.HAProxyRuntime[s.resource.Name]
	}
	if !ok {
		if s.resource.DNS != "" {
			return s.getExternalNameEndpoints()
		}
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
		HAProxySrvs: []*store.HAProxySrv{
			{
				Name:     "SRV_1",
				Address:  s.resource.DNS,
				Port:     port,
				Modified: true,
			},
		},
	}
	return endpoints, nil
}
