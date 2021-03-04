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
	"strconv"
	"strings"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Route struct {
	Namespace      *store.Namespace
	Ingress        *store.Ingress
	Path           *store.IngressPath
	service        *store.Service
	endpoints      *store.PortEndpoints
	status         store.Status
	HAProxyRules   []haproxy.RuleID
	Host           string
	BackendName    string
	NewBackend     bool
	LocalBackend   bool
	SSLPassthrough bool
	TCPService     bool
	reload         bool
}

// addToMapFile adds ingress route to haproxy Map files used for backend switching.
func (route *Route) addToMapFile(mapFiles haproxy.Maps) error {
	// Wildcard host
	if route.Host != "" && route.Host[0] == '*' {
		route.Host = route.Host[1:]
	}
	value := route.BackendName
	for _, id := range route.HAProxyRules {
		value += "." + strconv.Itoa(int(id))
	}
	// SSLPassthrough
	if route.SSLPassthrough {
		if route.Host == "" {
			return fmt.Errorf("empty SNI for backend %s, SKIP", route.BackendName)
		}
		mapFiles.AppendRow(SNI, route.Host+"\t\t\t"+value)
		return nil
	}
	// HTTP
	if route.Host != "" {
		mapFiles.AppendRow(HOST, route.Host+"\t\t\t"+route.Host)
	} else if route.Path.Path == "" {
		return fmt.Errorf("neither Host nor Path are provided for backend %v, SKIP", route.BackendName)
	}
	// if PathTypeExact is not set, PathTypePrefix will be applied
	path := route.Path.Path
	switch {
	case route.Path.ExactPathMatch:
		mapFiles.AppendRow(PATH_EXACT, route.Host+path+"\t\t\t"+value)
	case path == "" || path == "/":
		mapFiles.AppendRow(PATH_PREFIX, route.Host+"/"+"\t\t\t"+value)
	default:
		path = strings.TrimSuffix(path, "/")
		mapFiles.AppendRow(PATH_EXACT, route.Host+path+"\t\t\t"+value)
		mapFiles.AppendRow(PATH_PREFIX, route.Host+path+"/"+"\t\t\t"+value)
	}
	return nil
}

// handleBackend processes an Ingress Route and makes corresponding backend configuration in HAProxy
func (route *Route) handleBackend() (err error) {
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
	backendUpdated := annotations.HandleBackendAnnotations(
		k8sStore,
		client,
		&backend,
		route.NewBackend,
		route.service.Annotations,
		route.Ingress.Annotations,
		k8sStore.ConfigMaps[Main].Annotations,
	) || switchMode
	if backendUpdated {
		logger.Debugf("Ingress '%s/%s': Updating backend '%s'", route.Namespace.Name, route.Ingress.Name, route.BackendName)
		if err = client.BackendEdit(backend); err != nil {
			return err
		}
		route.reload = true
	}

	return nil
}

// SetBackendName checks if Ingress ServiceName and ServicePort exists and construct corresponding backend name
func (route *Route) SetBackendName() (err error) {
	route.service = route.Namespace.Services[route.Path.ServiceName]
	if route.service == nil {
		return fmt.Errorf("ingress %s/%s: service '%s' not found", route.Namespace.Name, route.Ingress.Name, route.Path.ServiceName)
	}
	route.Path.ResolvedSvcPort = ""
	for _, sp := range route.service.Ports {
		if sp.Name == route.Path.ServicePortString || sp.Port == route.Path.ServicePortInt {
			route.Path.ResolvedSvcPort = sp.Name
			break
		}
	}
	if route.Path.ResolvedSvcPort == "" {
		if route.Path.ServicePortInt != 0 {
			return fmt.Errorf("ingress %s/%s: service %s: no service port matching '%d'", route.Namespace.Name, route.Ingress.Name, route.service.Name, route.Path.ServicePortInt)
		}
		return fmt.Errorf("ingress %s/%s: service %s: no service port matching '%s'", route.Namespace.Name, route.Ingress.Name, route.service.Name, route.Path.ServicePortString)
	}
	route.BackendName = fmt.Sprintf("%s-%s-%s", route.service.Namespace, route.service.Name, route.Path.ResolvedSvcPort)
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
