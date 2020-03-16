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

package controller

import (
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models"
)

//Configuration represents k8s state

type NamespacesWatch struct {
	Whitelist map[string]struct{}
	Blacklist map[string]struct{}
}

type Configuration struct {
	Namespace              map[string]*Namespace
	NamespacesAccess       NamespacesWatch
	IngressClass           string
	ConfigMap              *ConfigMap
	ConfigMapTCPServices   *ConfigMap
	PublishService         *Service
	MapFiles               haproxy.Maps
	HTTPRequests           map[Rule]HTTPRequestRules
	HTTPRequestsStatus     Status
	TCPRequests            map[Rule]TCPRequestRules
	TCPRequestsStatus      Status
	BackendSwitchingRules  map[string]UseBackendRules
	BackendSwitchingStatus map[string]struct{}
	RateLimitingEnabled    bool
	HTTPS                  bool
	SSLPassthrough         bool
}

func (c *Configuration) IsRelevantNamespace(namespace string) bool {
	if namespace == "" {
		return false
	}
	if len(c.NamespacesAccess.Whitelist) > 0 {
		_, ok := c.NamespacesAccess.Whitelist[namespace]
		return ok
	}
	_, ok := c.NamespacesAccess.Blacklist[namespace]
	return !ok
}

//Init itialize configuration
func (c *Configuration) Init(osArgs utils.OSArgs, mapDir string) {

	c.NamespacesAccess = NamespacesWatch{
		Whitelist: map[string]struct{}{},
		Blacklist: map[string]struct{}{
			"kube-system": {},
		},
	}
	for _, namespace := range osArgs.NamespaceWhitelist {
		c.NamespacesAccess.Whitelist[namespace] = struct{}{}
	}
	for _, namespace := range osArgs.NamespaceBlacklist {
		c.NamespacesAccess.Blacklist[namespace] = struct{}{}
	}

	c.IngressClass = osArgs.IngressClass

	parts := strings.Split(osArgs.PublishService, "/")
	if len(parts) == 2 {
		c.PublishService = &Service{
			Namespace: parts[0],
			Name:      parts[1],
			Status:    EMPTY,
			Addresses: []string{},
		}
	}

	c.Namespace = make(map[string]*Namespace)

	c.HTTPRequests = make(map[Rule]HTTPRequestRules)
	for _, rule := range []Rule{SSL_REDIRECT, RATE_LIMIT, REQUEST_CAPTURE, REQUEST_SET_HEADER, WHITELIST} {
		c.HTTPRequests[rule] = make(map[uint64]models.HTTPRequestRule)
	}
	c.TCPRequests = make(map[Rule]TCPRequestRules)
	for _, rule := range []Rule{RATE_LIMIT, REQUEST_CAPTURE, PROXY_PROTOCOL, WHITELIST} {
		c.TCPRequests[rule] = make(map[uint64]models.TCPRequestRule)
	}
	c.HTTPRequestsStatus = EMPTY
	c.TCPRequestsStatus = EMPTY

	c.MapFiles = haproxy.NewMapFiles(mapDir)

	c.BackendSwitchingRules = make(map[string]UseBackendRules)
	c.BackendSwitchingStatus = make(map[string]struct{})
	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS, FrontendSSL} {
		c.BackendSwitchingRules[frontend] = UseBackendRules{}
	}
}

//GetNamespace returns Namespace. Creates one if not existing
func (c *Configuration) GetNamespace(name string) *Namespace {
	namespace, ok := c.Namespace[name]
	if ok {
		return namespace
	}
	newNamespace := c.NewNamespace(name)
	return newNamespace
}

//NewNamespace returns new initialized Namespace
func (c *Configuration) NewNamespace(name string) *Namespace {
	newNamespace := &Namespace{
		Name:      name,
		Relevant:  c.IsRelevantNamespace(name),
		Endpoints: make(map[string]*Endpoints),
		Services:  make(map[string]*Service),
		Ingresses: make(map[string]*Ingress),
		Secret:    make(map[string]*Secret),
		Status:    ADDED,
	}
	c.Namespace[name] = newNamespace
	return newNamespace
}

//Clean cleans all the statuses of various data that was changed
//deletes them completely or just resets them if needed
func (c *Configuration) Clean() {
	for _, namespace := range c.Namespace {
		for _, data := range namespace.Ingresses {
			for _, tls := range data.TLS {
				switch tls.Status {
				case DELETED:
					delete(data.TLS, tls.Host)
					continue
				default:
					tls.Status = EMPTY
				}
			}
			if data.DefaultBackend != nil {
				data.DefaultBackend.Status = EMPTY
			}
			for _, rule := range data.Rules {
				switch rule.Status {
				case DELETED:
					delete(data.Rules, rule.Host)
					continue
				default:
					rule.Status = EMPTY
					for _, path := range rule.Paths {
						switch path.Status {
						case DELETED:
							delete(rule.Paths, path.Path)
						default:
							path.Status = EMPTY
						}
					}
				}
			}
			data.Annotations.Clean()
			switch data.Status {
			case DELETED:
				delete(namespace.Ingresses, data.Name)
			default:
				data.Status = EMPTY
			}
		}
		for _, data := range namespace.Services {
			data.Annotations.Clean()
			switch data.Status {
			case DELETED:
				delete(namespace.Services, data.Name)
			default:
				data.Status = EMPTY
			}
		}
		for _, data := range namespace.Endpoints {
			switch data.Status {
			case DELETED:
				delete(namespace.Endpoints, data.Service.Value)
			default:
				data.Status = EMPTY
				for _, port := range *data.Ports {
					port.Status = EMPTY
				}
				for key, adr := range *data.Addresses {
					switch adr.Status {
					case DELETED:
						delete(*data.Addresses, key)
					default:
						adr.Status = EMPTY
					}
				}
				for _, adr := range *data.Addresses {
					adr.Status = EMPTY
				}
			}
		}
		for _, data := range namespace.Secret {
			switch data.Status {
			case DELETED:
				delete(namespace.Secret, data.Name)
			default:
				data.Status = EMPTY
			}
		}
	}
	c.ConfigMap.Annotations.Clean()
	switch c.ConfigMap.Status {
	case DELETED:
		c.ConfigMap = nil
	default:
		c.ConfigMap.Status = EMPTY
	}
	if c.ConfigMapTCPServices != nil {
		switch c.ConfigMapTCPServices.Status {
		case DELETED:
			c.ConfigMapTCPServices = nil
		default:
			c.ConfigMapTCPServices.Status = EMPTY
			c.ConfigMapTCPServices.Annotations.Clean()
		}
	}
	c.MapFiles.Clean()
	for rule := range c.HTTPRequests {
		c.HTTPRequests[rule] = make(map[uint64]models.HTTPRequestRule)
	}
	for rule := range c.TCPRequests {
		c.TCPRequests[rule] = make(map[uint64]models.TCPRequestRule)
	}
	c.HTTPRequestsStatus = EMPTY
	c.TCPRequestsStatus = EMPTY
	defaultAnnotationValues.Clean()
	if c.PublishService != nil {
		c.PublishService.Status = EMPTY
	}
}
