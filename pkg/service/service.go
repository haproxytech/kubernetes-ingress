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

	"github.com/haproxytech/client-native/v5/misc"
	"github.com/haproxytech/client-native/v5/models"

	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/rules/acls"
	"github.com/haproxytech/kubernetes-ingress/pkg/rules/httprequests"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetLogger()

const cookieKey = "ohph7OoGhong"

type Service struct {
	certs    certs.Certificates
	path     *store.IngressPath
	resource *store.Service
	backend  *models.Backend
	// ingressName      string
	// ingressNamespace string
	ingress       *store.Ingress
	annotations   []map[string]string
	modeTCP       bool
	newBackend    bool
	standalone    bool
	serversToEdit bool
}

// New returns a Service instance to handle the k8s IngressPath resource given in params.
// An error will be returned if there is no k8s Service resource corresponding to the service description in IngressPath.
func New(k store.K8s, path *store.IngressPath, certs certs.Certificates, tcpService bool, ingress *store.Ingress, annList ...map[string]string) (*Service, error) {
	service, err := k.GetService(path.SvcNamespace, path.SvcName)
	if err != nil {
		return nil, err
	}
	a := make([]map[string]string, 1, 3)
	a[0] = service.Annotations
	a = append(a, annList...)
	var standalone bool
	for _, anns := range a {
		standalone = len(anns) != 0 && anns["standalone-backend"] == "true"
		if standalone {
			break
		}
	}
	return &Service{
		path:        path,
		resource:    service,
		certs:       certs,
		annotations: a,
		modeTCP:     tcpService,
		standalone:  standalone,
		ingress:     ingress,
	}, nil
}

// NewLocal returns a Service instance to handle the k8s IngressPath resource given in params.
func NewLocal(k store.K8s, path *store.IngressPath, backend *models.Backend, annList ...map[string]string) (*Service, error) {
	return &Service{
		path: path,
		resource: &store.Service{
			Annotations: map[string]string{},
		},
		annotations: annList,
		backend:     backend,
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
	resourceNamespace := s.resource.Namespace
	resourceName := s.resource.Name
	prefix := ""
	if s.standalone && s.ingress != nil && s.ingress.Name != "" {
		resourceName = s.ingress.Name
		resourceNamespace = s.ingress.Namespace
		prefix = "ing_"
	}

	s.path.SvcPortResolved = &svcPort
	if svcPort.Name != "" {
		name = fmt.Sprintf("%s%s_%s_%s", prefix, resourceNamespace, resourceName, svcPort.Name)
	} else {
		name = fmt.Sprintf("%s%s_%s_%s", prefix, resourceNamespace, resourceName, strconv.Itoa(int(svcPort.Port)))
	}
	return
}

// HandleBackend processes a Service and creates/updates corresponding backend configuration in HAProxy
func (s *Service) HandleBackend(storeK8s store.K8s, client api.HAProxyClient, a annotations.Annotations) (err error) {
	var backend *models.Backend
	var newBackend *v1.BackendSpec
	newBackend, err = s.getBackendModel(storeK8s, a)
	if err != nil {
		s.backend = nil
		return
	}
	s.backend = newBackend.Config
	// Get/Create Backend
	backend, err = client.BackendGet(newBackend.Config.Name)
	if err == nil {
		// Update Backend
		diff := newBackend.Config.Diff(*backend)
		if len(diff) != 0 {
			// Detect if we have a diff on the server line
			if isServersToEdit(newBackend.Config, backend) {
				s.serversToEdit = true
			}
			if err = client.BackendEdit(*newBackend.Config); err != nil {
				return
			}
			instance.Reload("Service '%s/%s': backend '%s' updated: %v", s.resource.Namespace, s.resource.Name, newBackend.Config.Name, diff)
		}
	} else {
		if err = client.BackendCreate(*newBackend.Config); err != nil {
			return
		}
		s.newBackend = true
		instance.Reload(fmt.Sprintf("Service '%s/%s': new backend '%s'", s.resource.Namespace, s.resource.Name, newBackend.Config.Name))
	}
	// acls
	acls.PopulateBackend(client, newBackend.Config.Name, newBackend.Acls)
	// HTTP requests
	httprequests.PopulateBackend(client, newBackend.Config.Name, newBackend.HTTPRequests)
	// config-snippet: backend
	backendCfgSnippetHandler := annotations.NewCfgSnippet(
		annotations.ConfigSnippetOptions{
			Name:    "backend-config-snippet",
			Backend: utils.PtrString(newBackend.Config.Name),
			Ingress: s.ingress,
		})
	backendCfgSnippetHandler.SetService(s.resource)
	logger.Error(backendCfgSnippetHandler.Process(storeK8s, s.annotations...))
	return
}

func isServersToEdit(oldBackend *models.Backend, newBackend *models.Backend) bool {
	// Detect if we have a diff on the server line
	newCookie := newBackend.Cookie
	oldCookie := oldBackend.Cookie
	var cookieAreDifferent bool
	// Both are nil
	if newCookie == nil || oldCookie == nil {
		cookieAreDifferent = !(newCookie == oldCookie)
		return cookieAreDifferent
	}

	cookieAreDifferent = len(newCookie.Diff(*oldCookie)) > 0
	return cookieAreDifferent
}

// getBackendModel checks for a corresponding custom resource before falling back to annotations
func (s *Service) getBackendModel(store store.K8s, a annotations.Annotations) (backend *v1.BackendSpec, err error) {
	// Backend mode
	mode := "http"
	if s.modeTCP {
		mode = "tcp"
	}
	// get/create backend Model
	backend, err = annotations.ModelBackend("cr-backend", s.resource.Namespace, store, s.annotations...)
	logger.Warning(err)
	if backend == nil {
		backend = &v1.BackendSpec{
			Config: &models.Backend{Mode: mode},
		}
		for _, a := range a.Backend(backend.Config, store, s.certs) {
			err = a.Process(store, s.annotations...)
			if err != nil {
				logger.Errorf("service '%s/%s': annotation '%s': %s", s.resource.Namespace, s.resource.Name, a.GetName(), err)
			}
		}
	}

	// Manadatory backend params
	backend.Config.Mode = mode
	backend.Config.Name, err = s.GetBackendName()
	if err != nil {
		return nil, err
	}
	if s.resource.DNS != "" {
		if backend.Config.DefaultServer == nil {
			backend.Config.DefaultServer = &models.DefaultServer{ServerParams: models.ServerParams{InitAddr: misc.Ptr("last,libc,none")}}
		} else if backend.Config.DefaultServer.InitAddr == nil {
			backend.Config.DefaultServer.InitAddr = misc.Ptr("last,libc,none")
		}
	}
	if backend.Config.Cookie != nil && backend.Config.Cookie.Dynamic && backend.Config.DynamicCookieKey == "" {
		backend.Config.DynamicCookieKey = cookieKey
	}
	return backend, nil
}

// SetDefaultBackend configures the default service in kubernetes ingress resource as haproxy default backend of the frontends in params.
func (s *Service) SetDefaultBackend(k store.K8s, h haproxy.HAProxy, frontends []string, a annotations.Annotations) (err error) {
	if !s.path.IsDefaultBackend {
		err = fmt.Errorf("service '%s/%s' is not marked as default backend", s.resource.Namespace, s.resource.Name)
		return
	}
	var frontend models.Frontend
	frontend, err = h.FrontendGet(frontends[0])
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
	err = s.HandleBackend(k, h, a)
	if err != nil {
		return
	}
	backendName, _ := s.GetBackendName()
	if frontend.DefaultBackend != backendName {
		for _, frontendName := range frontends {
			frontend, _ := h.FrontendGet(frontendName)
			oldDefaultBackend := frontend.DefaultBackend
			frontend.DefaultBackend = backendName
			err = h.FrontendEdit(frontend)
			if err != nil {
				return
			}
			instance.Reload("default backend changed in frontend '%s': from '%s' to '%s'", frontendName, oldDefaultBackend, backendName)
		}
	}
	s.HandleHAProxySrvs(k, h)
	return
}
