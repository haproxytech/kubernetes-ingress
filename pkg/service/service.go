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

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/configuration"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetLogger()

const cookieKey = "ohph7OoGhong"

type Service struct {
	path        *store.IngressPath
	resource    *store.Service
	backend     *models.Backend
	certs       *certs.Certificates
	annotations []map[string]string
	modeTCP     bool
	newBackend  bool
}

// New returns a Service instance to handle the k8s IngressPath resource given in params.
// An error will be returned if there is no k8s Service resource corresponding to the service description in IngressPath.
func New(k store.K8s, path *store.IngressPath, certs *certs.Certificates, tcpService bool, annList ...map[string]string) (*Service, error) {
	service, err := k.GetService(path.SvcNamespace, path.SvcName)
	if err != nil {
		return nil, err
	}
	a := make([]map[string]string, 1, 3)
	a[0] = service.Annotations
	a = append(a, annList...)
	return &Service{
		path:        path,
		resource:    service,
		certs:       certs,
		annotations: a,
		modeTCP:     tcpService,
	}, nil
}

func (s *Service) GetResource() *store.Service {
	return s.resource
}

// GetBackendName checks if servicePort provided in IngressPath exists and construct corresponding backend name
// Backend name is in format "ServiceNS_ServiceName_PortName"
func (s *Service) GetBackendName() (name string, err error) {
	if s.backend != nil && s.backend.Name != "" {
		name = s.backend.Name
		return
	}
	var svcPort store.ServicePort
	found := false
	for _, sp := range s.resource.Ports {
		if (sp.Port == s.path.SvcPortInt) ||
			(sp.Name != "" && sp.Name == s.path.SvcPortString) {
			svcPort = sp
			found = true
			break
		}
	}
	if !found {
		if s.path.SvcPortString != "" {
			err = fmt.Errorf("service %s: no service port matching '%s'", s.resource.Name, s.path.SvcPortString)
		} else {
			err = fmt.Errorf("service %s: no service port matching '%d'", s.resource.Name, s.path.SvcPortInt)
		}
		return
	}
	s.path.SvcPortResolved = &svcPort
	if svcPort.Name != "" {
		name = fmt.Sprintf("%s_%s_%s", s.resource.Namespace, s.resource.Name, svcPort.Name)
	} else {
		name = fmt.Sprintf("%s_%s_%s", s.resource.Namespace, s.resource.Name, strconv.Itoa(int(svcPort.Port)))
	}
	return
}

// HandleBackend processes a Service and creates/updates corresponding backend configuration in HAProxy
func (s *Service) HandleBackend(client api.HAProxyClient, store store.K8s) (reload bool, err error) {
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
			logger.Debugf("Service '%s/%s': backend '%s' updated: %s\nReload required", s.resource.Namespace, s.resource.Name, newBackend.Name, result)
		}
	} else {
		if err = client.BackendCreate(*newBackend); err != nil {
			return
		}
		s.newBackend = true
		reload = true
		logger.Debugf("Service '%s/%s': new backend '%s', reload required", s.resource.Namespace, s.resource.Name, newBackend.Name)
	}
	// config-snippet
	logger.Error(annotations.NewBackendCfgSnippet("backend-config-snippet", newBackend.Name).Process(store, s.annotations...))
	change, errSnipp := annotations.UpdateBackendCfgSnippet(client, newBackend.Name)
	logger.Error(errSnipp)
	if len(change) != 0 {
		reload = true
		logger.Debugf("Service '%s/%s': backend '%s' config-snippet updated: %s\nReload required", s.resource.Namespace, s.resource.Name, newBackend.Name, change)
	}
	return
}

// getBackendModel checks for a corresponding custom resource before falling back to annoations
func (s *Service) getBackendModel(store store.K8s) (backend *models.Backend, err error) {
	// Backend mode
	var mode = "http"
	if s.modeTCP {
		mode = "tcp"
	}
	// get/create backend Model
	backend, err = annotations.ModelBackend("cr-backend", s.resource.Namespace, store, s.annotations...)
	logger.Warning(err)
	if backend == nil {
		backend = &models.Backend{Mode: mode}
		for _, a := range annotations.Backend(backend, store, s.certs) {
			err = a.Process(store, s.annotations...)
			if err != nil {
				logger.Errorf("service '%s/%s': annotation '%s': %s", s.resource.Namespace, s.resource.Name, a.GetName(), err)
			}
		}
	}

	// Manadatory backend params
	backend.Mode = mode
	backend.Name, err = s.GetBackendName()
	if err != nil {
		return nil, err
	}
	if s.resource.DNS != "" {
		if backend.DefaultServer == nil {
			backend.DefaultServer = &models.DefaultServer{InitAddr: "last,libc,none"}
		} else if backend.DefaultServer.InitAddr == "" {
			backend.DefaultServer.InitAddr = "last,libc,none"
		}
	}
	if backend.Cookie != nil && backend.Cookie.Dynamic && backend.DynamicCookieKey == "" {
		backend.DynamicCookieKey = cookieKey
	}
	return backend, nil
}

// SetDefaultBackend configures the default service in kubernetes ingress resource as haproxy default backend of the frontends in params.
func (s *Service) SetDefaultBackend(k store.K8s, cfg *configuration.ControllerCfg, api api.HAProxyClient, frontends []string) (reload bool, err error) {
	if !s.path.IsDefaultBackend {
		err = fmt.Errorf("service '%s/%s' is not marked as default backend", s.resource.Namespace, s.resource.Name)
		return
	}
	var frontend models.Frontend
	var ftReload bool
	frontend, err = api.FrontendGet(frontends[0])
	if err != nil {
		return
	}
	if frontend.Mode == "tcp" {
		s.modeTCP = true
	}
	// If port is not set in Ingress Path, use the first available port in service.
	if s.path.SvcPortInt == 0 && s.path.SvcPortString == "" {
		s.path.SvcPortString = s.resource.Ports[0].Name
	}
	bdReload, err := s.HandleBackend(api, k)
	if err != nil {
		return
	}
	backendName, _ := s.GetBackendName()
	if frontend.DefaultBackend != backendName {
		for _, frontendName := range frontends {
			frontend, _ := api.FrontendGet(frontendName)
			frontend.DefaultBackend = backendName
			err = api.FrontendEdit(frontend)
			if err != nil {
				return
			}
			ftReload = true
		}
	}
	cfg.ActiveBackends[backendName] = struct{}{}
	endpointsReload := s.HandleHAProxySrvs(api, k)
	reload = bdReload || ftReload || endpointsReload
	return reload, err
}
