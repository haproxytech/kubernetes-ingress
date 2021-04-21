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

package store

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type K8s struct {
	Namespaces       map[string]*Namespace
	IngressClasses   map[string]*IngressClass
	NamespacesAccess NamespacesWatch
	ConfigMaps       ConfigMaps
}

type NamespacesWatch struct {
	Whitelist map[string]struct{}
	Blacklist map[string]struct{}
}

var logger = utils.GetLogger()

func NewK8sStore(args utils.OSArgs) K8s {
	return K8s{
		Namespaces:     make(map[string]*Namespace),
		IngressClasses: make(map[string]*IngressClass),
		NamespacesAccess: NamespacesWatch{
			Whitelist: map[string]struct{}{},
			Blacklist: map[string]struct{}{},
		},
		ConfigMaps: ConfigMaps{
			Main: &ConfigMap{
				Namespace: args.ConfigMap.Namespace,
				Name:      args.ConfigMap.Name,
			},
			TCPServices: &ConfigMap{
				Namespace: args.ConfigMapTCPServices.Namespace,
				Name:      args.ConfigMapTCPServices.Name,
			},
			Errorfiles: &ConfigMap{
				Namespace: args.ConfigMapErrorFiles.Namespace,
				Name:      args.ConfigMapErrorFiles.Name,
			},
			PatternFiles: &ConfigMap{
				Namespace: args.ConfigMapPatternFiles.Namespace,
				Name:      args.ConfigMapPatternFiles.Name,
			},
		},
	}
}

func (k K8s) Clean() {
	for _, namespace := range k.Namespaces {
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
				for _, endpoints := range data.Ports {
					for _, srv := range endpoints.HAProxySrvs {
						srv.Modified = false
					}
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
	for _, cm := range []*ConfigMap{k.ConfigMaps.Main, k.ConfigMaps.TCPServices, k.ConfigMaps.Errorfiles} {
		switch cm.Status {
		case DELETED:
			cm.Status = DELETED
			cm.Annotations = MapStringW{}
		default:
			cm.Status = EMPTY
			cm.Annotations.Clean()
		}
	}
	for _, igClass := range k.IngressClasses {
		switch igClass.Status {
		case DELETED:
			delete(k.IngressClasses, igClass.Name)
		default:
			igClass.Status = EMPTY
		}
	}
	defaultAnnotationValues.Clean()
}

// GetNamespace returns Namespace. Creates one if not existing
func (k K8s) GetNamespace(name string) *Namespace {
	namespace, ok := k.Namespaces[name]
	if ok {
		return namespace
	}
	newNamespace := &Namespace{
		Name:      name,
		Relevant:  k.isRelevantNamespace(name),
		Endpoints: make(map[string]*Endpoints),
		Services:  make(map[string]*Service),
		Ingresses: make(map[string]*Ingress),
		Secret:    make(map[string]*Secret),
		Status:    ADDED,
	}
	k.Namespaces[name] = newNamespace
	return newNamespace
}

// FetchSecret fetches secret with secretPath format "namespace/secretName"
// if format is just "secretName" defaultNs param will be used.
func (k K8s) FetchSecret(secretPath, defaultNs string) (*Secret, error) {
	secretName := ""
	secretNamespace := defaultNs
	parts := strings.Split(secretPath, "/")
	if len(parts) > 1 {
		secretNamespace = parts[0]
		secretName = parts[1]
	} else {
		secretName = parts[0] // only secretname is here
	}
	ns, namespaceOK := k.Namespaces[secretNamespace]
	if !namespaceOK {
		return nil, fmt.Errorf("namespace '%s' does not exist", secretNamespace)
	}
	secret, secretOK := ns.Secret[secretName]
	if !secretOK {
		return nil, fmt.Errorf("secret '%s/%s' does not exist", secretNamespace, secretName)
	}
	return secret, nil
}

func (k K8s) isRelevantNamespace(namespace string) bool {
	if namespace == "" {
		return false
	}
	if len(k.NamespacesAccess.Whitelist) > 0 {
		_, ok := k.NamespacesAccess.Whitelist[namespace]
		return ok
	}
	_, ok := k.NamespacesAccess.Blacklist[namespace]
	return !ok
}
