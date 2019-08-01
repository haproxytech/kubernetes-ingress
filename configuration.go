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

package main

import (
	clientnative "github.com/haproxytech/client-native"
	"github.com/haproxytech/models"
)

const (
	//nolint
	RATE_LIMIT = "rate-limit"
	//nolint
	HTTP_REDIRECT = "http-redirect"
)

//Configuration represents k8s state

type NamespacesWatch struct {
	Whitelist map[string]struct{}
	Blacklist map[string]struct{}
}

type Configuration struct {
	Namespace             map[string]*Namespace
	NamespacesAccess      NamespacesWatch
	ConfigMap             *ConfigMap
	NativeAPI             *clientnative.HAProxyClient
	SSLRedirect           string
	RateLimitingEnabled   bool
	HTTPRequests          map[string][]models.HTTPRequestRule
	HTTPRequestsStatus    Status
	TCPRequests           map[string][]models.TCPRequestRule
	TCPRequestsStatus     Status
	UseBackendRules       map[string]BackendSwitchingRule
	UseBackendRulesStatus Status
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
func (c *Configuration) Init(osArgs OSArgs, api *clientnative.HAProxyClient) {

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
	c.Namespace = make(map[string]*Namespace)
	c.SSLRedirect = ""
	c.NativeAPI = api

	c.HTTPRequests = map[string][]models.HTTPRequestRule{}
	c.HTTPRequests[RATE_LIMIT] = []models.HTTPRequestRule{}
	c.HTTPRequestsStatus = EMPTY

	c.TCPRequests = map[string][]models.TCPRequestRule{}
	c.TCPRequests[RATE_LIMIT] = []models.TCPRequestRule{}
	c.TCPRequestsStatus = EMPTY

	c.HTTPRequests[HTTP_REDIRECT] = []models.HTTPRequestRule{}

	c.UseBackendRules = map[string]BackendSwitchingRule{}
	c.UseBackendRulesStatus = EMPTY
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
	c.HTTPRequestsStatus = EMPTY
	c.TCPRequestsStatus = EMPTY
	c.UseBackendRulesStatus = EMPTY
}
