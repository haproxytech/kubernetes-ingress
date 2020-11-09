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

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Route struct {
	Namespace          *store.Namespace
	Ingress            *store.Ingress
	Path               *store.IngressPath
	service            *store.Service
	endpoints          *store.PortEndpoints
	backendAnnotations map[string]*store.StringW
	srvAnnotations     map[string]*store.StringW
	status             store.Status
	Host               string
	BackendName        string
	NewBackend         bool
	SSLPassthrough     bool
	TCPService         bool
	reload             bool
}

// handleService processes an Ingress Route and make corresponding backend configuration in HAProxy
func (route *Route) handleService() (err error) {
	// Set backendName
	if route.Path.ServicePortInt == 0 {
		route.BackendName = fmt.Sprintf("%s-%s-%s", route.Namespace.Name, route.service.Name, route.Path.ServicePortString)
	} else {
		route.BackendName = fmt.Sprintf("%s-%s-%d", route.Namespace.Name, route.service.Name, route.Path.ServicePortInt)
	}

	// Get/Create Backend
	var backend models.Backend
	if backend, err = client.BackendGet(route.BackendName); err != nil {
		mode := "http"
		backend = models.Backend{
			Name: route.BackendName,
			Mode: mode,
		}
		logger.Debugf("Ingress '%s/%s': Creating new backend '%s'", route.Namespace.Name, route.Ingress.Name, route.BackendName)
		if err = client.BackendCreate(backend); err != nil {
			return err
		}
		route.NewBackend = true
		route.reload = true
	}

	// Update Backend
	var switchMode bool
	if backend.Mode == "http" {
		if route.SSLPassthrough || route.TCPService {
			backend.Mode = "tcp"
			switchMode = true
		}
	} else if !route.SSLPassthrough && !route.TCPService {
		backend.Mode = "http"
		switchMode = true
	}
	if route.handleBackendAnnotations(&backend) || switchMode {
		logger.Debugf("Ingress '%s/%s': Updating backend '%s'", route.Namespace.Name, route.Ingress.Name, route.BackendName)
		if err = client.BackendEdit(backend); err != nil {
			return err
		}
		route.reload = true
	}

	return nil
}

func (route *Route) setStatus() {
	if route.Path.Status == DELETED || route.service.Status == DELETED {
		route.status = DELETED
		return
	}
	route.status = route.Path.Status
	if route.status == EMPTY {
		route.status = route.service.Status
	}
}
