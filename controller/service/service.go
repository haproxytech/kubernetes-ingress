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

	"github.com/go-test/deep"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

var logger = utils.GetLogger()

type SvcContext struct {
	store      store.K8s
	ingress    *store.Ingress
	path       *store.IngressPath
	service    *store.Service
	backend    *models.Backend
	certs      *haproxy.Certificates
	modeTCP    bool
	newBackend bool
}

func NewCtx(k8s store.K8s, ingress *store.Ingress, path *store.IngressPath, certs *haproxy.Certificates, tcpService bool) (*SvcContext, error) {
	service, err := getService(k8s, ingress.Namespace, path.SvcName)
	if err != nil {
		return nil, err
	}
	return &SvcContext{
		store:   k8s,
		ingress: ingress,
		path:    path,
		service: service,
		certs:   certs,
		modeTCP: tcpService,
	}, nil
}

func (s *SvcContext) GetStatus() store.Status {
	if s.path.Status == store.DELETED || s.service.Status == store.DELETED {
		return store.DELETED
	}
	if s.path.Status == store.EMPTY {
		return s.service.Status
	}
	return s.path.Status
}

func (s *SvcContext) GetService() *store.Service {
	return s.service
}

// GetBackendName checks if servicePort provided in IngressPath exists and construct corresponding backend name
// Backend name is in format "ServiceNS-ServiceName-PortName"
func (s *SvcContext) GetBackendName() (name string, err error) {
	if s.backend != nil && s.backend.Name != "" {
		name = s.backend.Name
		return
	}
	var svcPort store.ServicePort
	found := false
	for _, sp := range s.service.Ports {
		if (sp.Port == s.path.SvcPortInt) ||
			(sp.Name != "" && sp.Name == s.path.SvcPortString) {
			svcPort = sp
			found = true
			break
		}
	}
	if !found {
		if s.path.SvcPortString != "" {
			err = fmt.Errorf("service %s: no service port matching '%s'", s.service.Name, s.path.SvcPortString)
		} else {
			err = fmt.Errorf("service %s: no service port matching '%d'", s.service.Name, s.path.SvcPortInt)
		}
		return
	}
	s.path.SvcPortResolved = &svcPort
	if svcPort.Name != "" {
		name = fmt.Sprintf("%s-%s-%s", s.service.Namespace, s.service.Name, svcPort.Name)
	} else {
		name = fmt.Sprintf("%s-%s-%s", s.service.Namespace, s.service.Name, strconv.Itoa(int(svcPort.Port)))
	}
	return
}

// HandleBackend processes a Service Context and creates/updates corresponding backend configuration in HAProxy
func (s *SvcContext) HandleBackend(client api.HAProxyClient, store store.K8s) (reload bool, err error) {
	var backend, newBackend *models.Backend
	newBackend, err = s.getBackendModel(store)
	s.backend = newBackend
	if err != nil {
		return
	}
	// Get/Create Backend
	backend, err = client.BackendGet(newBackend.Name)
	if err == nil {
		// Update Backend
		result := deep.Equal(newBackend, backend)
		if len(result) != 0 {
			if err = client.BackendEdit(*newBackend); err != nil {
				return
			}
			reload = true
			logger.Debugf("Ingress '%s/%s': backend '%s' updated: %s\nReload required", s.ingress.Namespace, s.ingress.Name, newBackend.Name, result)
		}
	} else {
		if err = client.BackendCreate(*newBackend); err != nil {
			return
		}
		s.newBackend = true
		reload = true
		logger.Debugf("Ingress '%s/%s': new backend '%s', reload required", s.ingress.Namespace, s.ingress.Name, newBackend.Name)
	}
	// config-snippet
	logger.Error(annotations.NewBackendCfgSnippet("backend-config-snippet", newBackend.Name).Process(store, s.service.Annotations, s.ingress.Annotations, store.ConfigMaps.Main.Annotations))
	change, errSnipp := annotations.UpdateBackendCfgSnippet(client, newBackend.Name)
	logger.Error(errSnipp)
	if len(change) != 0 {
		reload = true
		logger.Debugf("Ingress '%s/%s': backend '%s' updated: %s\nReload required", s.ingress.Namespace, s.ingress.Name, newBackend.Name, change)
	}
	return
}

// getBackendModel checks for a corresponding custom resource before falling back to annoations
func (s *SvcContext) getBackendModel(store store.K8s) (*models.Backend, error) {
	var backend *models.Backend
	var err error
	var cookieKey = "ohph7OoGhong"
	crInuse := true
	backend, err = annotations.ModelBackend("cr-backend", s.service.Namespace, store, s.service.Annotations, s.ingress.Annotations, store.ConfigMaps.Main.Annotations)
	logger.Warning(err)
	if backend == nil {
		backend = &models.Backend{DefaultServer: &models.DefaultServer{}}
		crInuse = false
	}
	if !crInuse {
		for _, a := range annotations.Backend(backend, store, s.certs) {
			err = a.Process(store, s.service.Annotations, s.ingress.Annotations, store.ConfigMaps.Main.Annotations)
			if err != nil {
				logger.Errorf("service '%s/%s': annotation '%s': %s", s.service.Namespace, s.service.Name, a.GetName(), err)
			}
		}
	}
	if s.modeTCP {
		backend.Mode = "tcp"
	} else {
		backend.Mode = "http"
	}
	if backend.Name, err = s.GetBackendName(); err != nil {
		return nil, err
	}
	if s.service.DNS != "" {
		backend.DefaultServer = &models.DefaultServer{InitAddr: "last,libc,none"}
	}
	if backend.Cookie != nil && backend.Cookie.Dynamic && backend.DynamicCookieKey == "" {
		backend.DynamicCookieKey = cookieKey
	}
	return backend, nil
}

func getService(k8s store.K8s, namespace, name string) (*store.Service, error) {
	var service *store.Service
	ns, ok := k8s.Namespaces[namespace]
	if !ok {
		return nil, fmt.Errorf("service '%s/%s' namespace not found", namespace, name)
	}
	service, ok = ns.Services[name]
	if !ok {
		return nil, fmt.Errorf("service '%s/%s' not found", namespace, name)
	}
	return service, nil
}
