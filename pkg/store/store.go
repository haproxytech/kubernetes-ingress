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

	"github.com/haproxytech/client-native/v3/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

const DefaultLocalBackend = "default-local-service"

type K8s struct {
	ConfigMaps                   ConfigMaps
	NamespacesAccess             NamespacesWatch
	Namespaces                   map[string]*Namespace
	IngressClasses               map[string]*IngressClass
	SecretsProcessed             map[string]struct{}
	BackendsProcessed            map[string]struct{}
	GatewayClasses               map[string]*GatewayClass
	GatewayControllerName        string
	PublishServiceAddresses      []string
	NbrHAProxyInst               int64
	UpdateAllIngresses           bool
	BackendsWithNoConfigSnippets map[string]struct{}
}

type NamespacesWatch struct {
	Whitelist map[string]struct{}
	Blacklist map[string]struct{}
}

type ErrNotFound error

var logger = utils.GetLogger()

func NewK8sStore(args utils.OSArgs) K8s {
	store := K8s{
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
		SecretsProcessed:             map[string]struct{}{},
		BackendsProcessed:            map[string]struct{}{},
		GatewayClasses:               map[string]*GatewayClass{},
		BackendsWithNoConfigSnippets: map[string]struct{}{},
	}
	for _, namespace := range args.NamespaceWhitelist {
		store.NamespacesAccess.Whitelist[namespace] = struct{}{}
	}
	for _, namespace := range args.NamespaceBlacklist {
		store.NamespacesAccess.Blacklist[namespace] = struct{}{}
	}
	return store
}

func (k *K8s) Clean() {
	for _, namespace := range k.Namespaces {
		for _, data := range namespace.Ingresses {
			data.Status = EMPTY
		}
		for _, data := range namespace.Services {
			switch data.Status {
			case DELETED:
				delete(namespace.Services, data.Name)
			default:
				data.Status = EMPTY
			}
		}
		for _, serviceEndpointSlices := range namespace.Endpoints {
			for _, slice := range serviceEndpointSlices {
				switch slice.Status {
				case DELETED:
					delete(namespace.Endpoints[slice.Service], slice.SliceName)
					if len(namespace.Endpoints[slice.Service]) == 0 {
						delete(namespace.Endpoints, slice.Service)
						delete(namespace.HAProxyRuntime, slice.Service)
					}
				default:
					slice.Status = EMPTY
					for _, backend := range namespace.HAProxyRuntime[slice.Service] {
						for _, srv := range backend.HAProxySrvs {
							srv.Modified = false
						}
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
	for _, igClass := range k.IngressClasses {
		igClass.Status = EMPTY
	}
	for _, cm := range []*ConfigMap{k.ConfigMaps.Main, k.ConfigMaps.TCPServices, k.ConfigMaps.Errorfiles} {
		switch cm.Status {
		case DELETED:
			cm.Status = DELETED
			cm.Annotations = map[string]string{}
		default:
			cm.Status = EMPTY
		}
	}
	k.SecretsProcessed = map[string]struct{}{}
}

// GetNamespace returns Namespace. Creates one if not existing
func (k K8s) GetNamespace(name string) *Namespace {
	namespace, ok := k.Namespaces[name]
	if ok {
		return namespace
	}
	newNamespace := &Namespace{
		Name:           name,
		Relevant:       k.isRelevantNamespace(name),
		Endpoints:      make(map[string]map[string]*Endpoints),
		Services:       make(map[string]*Service),
		Ingresses:      make(map[string]*Ingress),
		Secret:         make(map[string]*Secret),
		HAProxyRuntime: make(map[string]map[string]*RuntimeBackend),
		CRs: &CustomResources{
			Global:     make(map[string]*models.Global),
			Defaults:   make(map[string]*models.Defaults),
			LogTargets: make(map[string]models.LogTargets),
			Backends:   make(map[string]*models.Backend),
		},
		Gateways:        make(map[string]*Gateway),
		TCPRoutes:       make(map[string]*TCPRoute),
		ReferenceGrants: make(map[string]*ReferenceGrant),
		Labels:          make(map[string]string),
		Status:          ADDED,
	}
	k.Namespaces[name] = newNamespace
	return newNamespace
}

func (k K8s) GetSecret(namespace, name string) (*Secret, error) {
	ns, ok := k.Namespaces[namespace]
	if !ok {
		return nil, fmt.Errorf("secret '%s/%s' does not exist, namespace not found", namespace, name)
	}
	secret, secretOK := ns.Secret[name]
	if !secretOK {
		return nil, ErrNotFound(fmt.Errorf("secret '%s/%s' does not exist", namespace, name))
	}
	if secret.Status == DELETED {
		return nil, ErrNotFound(fmt.Errorf("secret '%s/%s' deleted", namespace, name))
	}
	return secret, nil
}

func (k K8s) GetService(namespace, name string) (*Service, error) {
	ns, nsOk := k.Namespaces[namespace]
	if !nsOk {
		return nil, fmt.Errorf("service '%s/%s' does not exist, namespace not found", namespace, name)
	}
	svc, svcOk := ns.Services[name]
	if !svcOk {
		return nil, ErrNotFound(fmt.Errorf("service '%s/%s' does not exist", namespace, name))
	}
	if svc.Status == DELETED {
		return nil, ErrNotFound(fmt.Errorf("service '%s/%s' deleted", namespace, name))
	}
	return svc, nil
}

// GetEndpoints takes the ns and name of a service and provides a map of endpoints: portName --> *PortEndpoints
func (k K8s) GetEndpoints(namespace, name string) (endpoints map[string]*PortEndpoints, err error) {
	ns, nsOk := k.Namespaces[namespace]
	if !nsOk {
		return nil, fmt.Errorf("service '%s/%s' does not exist, namespace not found", namespace, name)
	}
	slices, ok := ns.Endpoints[name]
	if !ok {
		return nil, fmt.Errorf("endpoints for service '%s/%s', does not exist", namespace, name)
	}
	endpoints = make(map[string]*PortEndpoints)
	for sliceName := range slices {
		for portName, portEndpoints := range slices[sliceName].Ports {
			endpoints[portName] = portEndpoints
		}
	}
	return
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
