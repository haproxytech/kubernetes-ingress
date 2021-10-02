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
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

var logger = utils.GetLogger()

type SvcContext struct {
	store       store.K8s
	ingress     *store.Ingress
	path        *store.IngressPath
	service     *store.Service
	tcpService  bool
	newBackend  bool
	backendName string
}

func NewCtx(k8s store.K8s, ingress *store.Ingress, path *store.IngressPath, tcpService bool) (*SvcContext, error) {
	service, err := getService(k8s, ingress.Namespace, path.SvcName)
	if err != nil {
		return nil, err
	}
	return &SvcContext{
		store:      k8s,
		ingress:    ingress,
		path:       path,
		service:    service,
		tcpService: tcpService,
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
func (s *SvcContext) GetBackendName() (string, error) {
	if s.backendName != "" {
		return s.backendName, nil
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
			return "", fmt.Errorf("service %s: no service port matching '%s'", s.service.Name, s.path.SvcPortString)
		}
		return "", fmt.Errorf("service %s: no service port matching '%d'", s.service.Name, s.path.SvcPortInt)
	}
	s.path.SvcPortResolved = &svcPort
	if svcPort.Name != "" {
		s.backendName = fmt.Sprintf("%s-%s-%s", s.service.Namespace, s.service.Name, svcPort.Name)
	} else {
		s.backendName = fmt.Sprintf("%s-%s-%s", s.service.Namespace, s.service.Name, strconv.Itoa(int(svcPort.Port)))
	}
	return s.backendName, nil
}

// HandleBackend processes a Service Context and creates/updates corresponding backend configuration in HAProxy
func (s *SvcContext) HandleBackend(client api.HAProxyClient, store store.K8s) (reload bool, backendName string, err error) {
	if backendName, err = s.GetBackendName(); err != nil {
		return reload, backendName, err
	}
	var backend, oldBackend *models.Backend
	// Get/Create Backend
	backend = &models.Backend{
		Name: backendName,
		Mode: "http",
	}
	if s.service.DNS != "" {
		backend.DefaultServer = &models.DefaultServer{InitAddr: "last,libc,none"}
	}
	if s.tcpService {
		backend.Mode = "tcp"
	}
	oldBackend, err = client.BackendGet(backendName)
	if err != nil {
		if err = client.BackendCreate(*backend); err != nil {
			return reload, backendName, err
		}
		s.newBackend = true
		reload = true
		logger.Debugf("Ingress '%s/%s': new backend '%s', reload required", s.ingress.Namespace, s.ingress.Name, backendName)
	}
	for _, a := range annotations.Backend(backend) {
		annValue := annotations.GetValue(a.GetName(), s.service.Annotations, s.ingress.Annotations, store.ConfigMaps.Main.Annotations)
		err = a.Process(annValue)
		if err != nil {
			logger.Errorf("service '%s/%s': annotation '%s': %s", s.service.Namespace, s.service.Name, a.GetName(), err)
		}
	}
	// Update Backend
	result := deep.Equal(oldBackend, backend)
	if len(result) != 0 {
		if err = client.BackendEdit(*backend); err != nil {
			return reload, backendName, err
		}
		reload = true
		logger.Debugf("Ingress '%s/%s': backend '%s' updated: %s\nReload required", s.ingress.Namespace, s.ingress.Name, backend.Name, result)
	}
	change, errSnipp := annotations.UpdateBackendCfgSnippet(client, backend.Name)
	logger.Error(errSnipp)
	reload = reload || change

	return reload, backendName, nil
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
