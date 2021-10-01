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

	"github.com/go-test/deep"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

// HandleEndpoints lookups the IngressPath related endpoints and handles corresponding backend servers configuration in HAProxy
func (s *SvcContext) HandleEndpoints(client api.HAProxyClient, k8sStore store.K8s, certs *haproxy.Certificates) (reload bool) {
	var srvsScaled, srvsActiveAnn bool

	ns := k8sStore.Namespaces[s.service.Namespace]
	if ns == nil {
		logger.Warningf("Ingress '%s/%s': Not found", s.ingress.Namespace, s.ingress.Name)
		return
	}
	sp := s.path.SvcPortResolved
	newAddresses, HAProxySrvs, err := s.getEndpointConfiguration(ns)

	if err != nil {
		logger.Warningf("Ingress '%s/%s': %s", s.ingress.Namespace, s.ingress.Name, err)
		return
	}
	// set backendName in store for runtime updates.
	ns.HAProxyConfig[s.service.Name].BackendName[sp.Name] = s.backendName

	// scale servers
	if s.service.DNS == "" {
		srvsScaled = s.scaleHAProxySrvs(newAddresses, HAProxySrvs, k8sStore)
	}
	// update servers
	srv, _ := client.ServerGet("SRV_1", s.backendName)
	srvsActiveAnn = s.handleSrvAnnotations(&srv, k8sStore, certs)
	for _, srvSlot := range *HAProxySrvs {
		if srvSlot.Modified || srvsActiveAnn {
			s.updateHAProxySrv(client, srv, *srvSlot, srvSlot.Port)
		}
	}

	if ns.HAProxyConfig[s.service.Name].DynUpdateFailed[sp.Name] {
		ns.HAProxyConfig[s.service.Name].DynUpdateFailed[sp.Name] = false
		return true
	}

	return srvsScaled || srvsActiveAnn
}

func (s *SvcContext) handleSrvAnnotations(srv *models.Server, store store.K8s, certs *haproxy.Certificates) bool {
	var err error
	oldSrv := *srv
	for _, a := range annotations.GetServerAnnotations(srv, store, certs) {
		annValue := annotations.GetValue(a.GetName(), s.service.Annotations, s.ingress.Annotations, s.store.ConfigMaps.Main.Annotations)
		err = a.Process(annValue)
		if err != nil {
			logger.Errorf("service %s/%s: annotation '%s': %s", s.service.Namespace, s.service.Name, a.GetName(), err)
		}
	}
	if s.newBackend {
		return true
	}
	result := deep.Equal(&oldSrv, srv)
	if len(result) != 0 {
		logger.Debugf("Ingress '%s/%s': server options for backend '%s' were updated:%s\nReload required", s.ingress.Namespace, s.ingress.Name, s.backendName, result)
		return true
	}
	return false
}

// updateHAProxySrv updates corresponding HAProxy backend server or creates one if it does not exist
func (s *SvcContext) updateHAProxySrv(client api.HAProxyClient, srv models.Server, srvSlot store.HAProxySrv, port int64) {
	srv.Name = srvSlot.Name
	srv.Port = &port
	// Enabled/Disabled
	if srvSlot.Address == "" {
		srv.Address = "127.0.0.1"
		srv.Maintenance = "enabled"
	} else {
		srv.Address = srvSlot.Address
		srv.Maintenance = "disabled"
	}
	// Update server
	errAPI := client.BackendServerEdit(s.backendName, srv)
	if errAPI == nil {
		logger.Tracef("Updating server '%s/%s' %d", s.backendName, srv.Name, srv.Port)
		return
	}
	// Create server
	if strings.Contains(errAPI.Error(), "does not exist") {
		logger.Tracef("Updating server '%s/%s' %d", s.backendName, srv.Name, srv.Port)
		logger.Error(client.BackendServerCreate(s.backendName, srv))
	}
}

// scaleHAproxySrvs adds servers to match available addresses
func (s *SvcContext) scaleHAProxySrvs(newAddresses *map[string]*store.Address, haProxySrvs *[]*store.HAProxySrv, k8sStore store.K8s) (reload bool) {
	var flag bool
	var srvSlots int
	var disabled []*store.HAProxySrv
	// Add disabled HAProxySrvs to match "scale-server-slots"
	// scale-server-slots has a default value in defaultAnnotations
	// "servers-increment", "server-slots" are legacy annotations
	for _, annotation := range []string{"servers-increment", "server-slots", "scale-server-slots"} {
		annServerSlots := annotations.GetValue(annotation, k8sStore.ConfigMaps.Main.Annotations)
		if annServerSlots != "" {
			if value, err := strconv.Atoi(annServerSlots); err == nil {
				srvSlots = value
				break
			} else {
				logger.Error(err)
			}
		}
	}

	for len(*haProxySrvs) < srvSlots {
		srv := &store.HAProxySrv{
			Name:     fmt.Sprintf("SRV_%d", len(*haProxySrvs)+1),
			Address:  "",
			Modified: true,
			Port:     1,
		}
		*haProxySrvs = append(*haProxySrvs, srv)
		disabled = append(disabled, srv)
		flag = true
	}
	if flag {
		reload = true
		logger.Debugf("Server slots in backend '%s' scaled to match scale-server-slots value: %d, reload required", s.backendName, srvSlots)
	}
	// Configure remaining addresses in available haProxySrvs
	flag = false
	for addr, Address := range *newAddresses {
		if len(disabled) != 0 {
			disabled[0].Address = addr
			disabled[0].Modified = true
			disabled[0].Port = Address.Port
			disabled = disabled[1:]
		} else {
			srv := &store.HAProxySrv{
				Name:     fmt.Sprintf("SRV_%d", len(*haProxySrvs)+1),
				Address:  addr,
				Modified: true,
				Port:     Address.Port,
			}
			*haProxySrvs = append(*haProxySrvs, srv)
			flag = true
		}
		delete(*newAddresses, addr)
	}
	if flag {
		reload = true
		logger.Debugf("Server slots in backend '%s' scaled to match available endpoints, reload required", s.backendName)
	}
	return reload
}

func (s *SvcContext) getEndpointConfiguration(ns *store.Namespace) (newAddresses *map[string]*store.Address, haProxySrvs *[]*store.HAProxySrv, err error) {
	sp := s.path.SvcPortResolved

	if ns.HAProxyConfig[s.service.Name] == nil {
		ns.HAProxyConfig[s.service.Name] = &store.HAProxyConfig{
			HAProxySrvs:  make(map[string]*[]*store.HAProxySrv),
			NewAddresses: make(map[string]map[string]*store.Address),
			BackendName:  make(map[string]string),
		}
	}
	isExternalNameService := s.service.DNS != ""
	if isExternalNameService {
		var err error
		haProxySrvs, err = s.getExternalNameEndpoints()
		if err != nil {
			return nil, nil, err
		}
		return nil, haProxySrvs, nil
	}

	addresses := ns.HAProxyConfig[s.service.Name].NewAddresses[sp.Name]

	if ns.HAProxyConfig[s.service.Name].HAProxySrvs[sp.Name] == nil {
		tmp := make([]*store.HAProxySrv, 0, len(addresses))
		ns.HAProxyConfig[s.service.Name].HAProxySrvs[sp.Name] = &tmp
	}

	haProxySrvs = ns.HAProxyConfig[s.service.Name].HAProxySrvs[sp.Name]

	if len(addresses) == 0 && len(*haProxySrvs) == 0 {
		if s.path.SvcPortString != "" {
			return nil, nil, fmt.Errorf("no matching endpoints for service '%s' and port '%s'", s.service.Name, s.path.SvcPortString)
		}
		return nil, nil, fmt.Errorf("no matching endpoints for service '%s' and port '%d'", s.service.Name, s.path.SvcPortInt)
	}

	return &addresses, haProxySrvs, nil
}

func (s *SvcContext) getExternalNameEndpoints() (haProxySrvs *[]*store.HAProxySrv, err error) {
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

	srvs := []*store.HAProxySrv{
		{
			Name:     "SRV_1",
			Address:  s.service.DNS,
			Modified: true,
			Port:     port,
		},
	}

	return &srvs, nil
}
