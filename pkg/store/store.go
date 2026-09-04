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
	"path/filepath"
	"strings"

	"github.com/haproxytech/client-native/v6/models"
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	rc "github.com/haproxytech/kubernetes-ingress/pkg/reference-counter"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

const (
	DefaultLocalBackend = "default-local-service"
	CONTROLLER          = "haproxy.org/ingress-controller"
)

type K8s struct {
	ConfigMaps        ConfigMaps
	NamespacesAccess  NamespacesWatch
	Namespaces        map[string]*Namespace
	IngressClasses    map[string]*IngressClass
	SecretsProcessed  map[string]struct{}
	BackendsProcessed map[string]BackendOwner // backend name -> ingress which built it
	// RoutesProcessedByMapFile records, per map file, which ingress declared each of its keys
	// during the current reconciliation. Reset at every reconciliation, like BackendsProcessed.
	//
	// The outer key is the name of the map file - "sni", "host", "path-exact",
	// "path-prefix-exact", "path-prefix". A plain string and not a maps.Name, because
	// pkg/haproxy/maps transitively imports this package.
	//
	// The inner key is the key haproxy looks up in that file, and its composition is not the
	// same from one file to the next:
	//
	//   - "sni" and "host": the host alone. The path is not part of the key, so every path of
	//     a host lands on the same one - which is why a passthrough host cannot serve two
	//     paths through two different backends.
	//   - "path-exact", "path-prefix-exact", "path-prefix": the host concatenated with the
	//     path, normalised - a trailing slash trimmed or added depending on the path type, and
	//     the host alone followed by a slash when the path is empty or "/".
	//
	// Those keys are not built here: they come from route.MapRows, which is the single
	// description of the rows a route produces, and which normalises them. Anything reasoning
	// about a key has to take it from there rather than rebuild it, since the same declaration
	// can produce keys in several files and two different path types can produce the same key.
	//
	// Nesting per map file rather than joining the two into one string is what avoids reserving
	// a separator that cannot appear in a host or a path.
	RoutesProcessedByMapFile     map[string]map[string]RouteOwner
	GatewayClasses               map[string]*GatewayClass
	HaProxyPods                  map[string]struct{}
	BackendsWithNoConfigSnippets map[string]struct{}
	FrontendRC                   *rc.ResourceCounter
	GatewayControllerName        string
	PublishServiceAddresses      []string
	publishServiceNS             string
	publishServiceName           string
	UpdateAllIngresses           bool
	IngressesByService           map[string]*utils.OrderedSet[string, *Ingress] // service fqn -> ingress name -> ingress
}

type NamespacesWatch struct {
	Whitelist           map[string]struct{}
	Blacklist           map[string]struct{}
	Selected            map[string]struct{}
	LabelSelectorActive bool
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
			Selected:  map[string]struct{}{},
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
		BackendsProcessed:            map[string]BackendOwner{},
		RoutesProcessedByMapFile:     map[string]map[string]RouteOwner{},
		GatewayClasses:               map[string]*GatewayClass{},
		BackendsWithNoConfigSnippets: map[string]struct{}{},
		HaProxyPods:                  map[string]struct{}{},
		FrontendRC:                   rc.NewResourceCounter(),
		IngressesByService:           map[string]*utils.OrderedSet[string, *Ingress]{},
	}
	for _, namespace := range args.NamespaceWhitelist {
		store.NamespacesAccess.Whitelist[namespace] = struct{}{}
	}
	for _, namespace := range args.NamespaceBlacklist {
		store.NamespacesAccess.Blacklist[namespace] = struct{}{}
	}
	if args.NamespaceLabelSelectorActive() {
		store.NamespacesAccess.LabelSelectorActive = true
	}
	if parts := strings.Split(args.PublishService, "/"); len(parts) == 2 {
		store.publishServiceNS = parts[0]
		store.publishServiceName = parts[1]
	}
	return store
}

func (k *K8s) Clean() {
	for _, namespace := range k.Namespaces {
		for _, data := range namespace.Ingresses {
			data.Status = EMPTY
			data.ClassUpdated = false
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
						delete(namespace.HAProxyRuntimeStandalone, slice.Service)
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
		for _, cr := range namespace.CRs.TCPsPerCR {
			switch cr.Status {
			case DELETED:
				delete(namespace.CRs.TCPsPerCR, cr.Name)
			default:
				cr.Status = EMPTY
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
	// Same nature as the Status fields above: a publish service event raises it to ask for
	// every ingress to be published again, and the sync which did it has consumed it. It
	// cannot be reset by the status manager, which receives the store by value.
	//
	// Being reset here also means a failed sync keeps it, clean(failedSync) skipping this
	// whole function: the sweep is retried instead of being lost.
	k.UpdateAllIngresses = false
}

func newEmptyNamespace(name string, relevant bool) *Namespace {
	return &Namespace{
		Name:                     name,
		Relevant:                 relevant,
		Endpoints:                make(map[string]map[string]*Endpoints),
		Services:                 make(map[string]*Service),
		Ingresses:                make(map[string]*Ingress),
		Secret:                   make(map[string]*Secret),
		HAProxyRuntime:           make(map[string]map[string]*RuntimeBackend),
		HAProxyRuntimeStandalone: make(map[string]map[string]map[string]*RuntimeBackend),
		CRs: &CustomResources{
			Global:    make(map[string]*models.Global),
			Defaults:  make(map[string]*models.Defaults),
			Backends:  make(map[string]*v3.BackendSpec),
			TCPsPerCR: make(map[string]*TCPs),
			Frontends: make(map[string]*v3.FrontendSpec),
		},
		Gateways:        make(map[string]*Gateway),
		TCPRoutes:       make(map[string]*TCPRoute),
		ReferenceGrants: make(map[string]*ReferenceGrant),
		Labels:          make(map[string]string),
		Status:          ADDED,
	}
}

// GetNamespace returns Namespace. Creates one if not existing, except when
// label-selector mode is active and the name is not currently selected.
func (k K8s) GetNamespace(name string) *Namespace {
	namespace, ok := k.Namespaces[name]
	if ok {
		return namespace
	}
	if k.NamespacesAccess.LabelSelectorActive {
		if _, selected := k.NamespacesAccess.Selected[name]; !selected {
			return newEmptyNamespace(name, false)
		}
		newNamespace := newEmptyNamespace(name, false)
		k.Namespaces[name] = newNamespace
		return newNamespace
	}
	newNamespace := newEmptyNamespace(name, k.isRelevantNamespace(name))
	k.Namespaces[name] = newNamespace
	return newNamespace
}

// MarkNamespaceReady marks a selected namespace as ready for config generation.
func (k *K8s) MarkNamespaceReady(name string) bool {
	if !k.NamespacesAccess.LabelSelectorActive {
		return false
	}
	if _, ok := k.NamespacesAccess.Selected[name]; !ok {
		return false
	}
	ns, ok := k.Namespaces[name]
	if !ok {
		return false
	}
	if ns.Relevant {
		return false
	}
	ns.Relevant = true
	k.checkCollisionsAllNamespaces()
	return true
}

func (k K8s) selectorNamespacePersistent(ns *Namespace) bool {
	if ns == nil {
		return false
	}
	stored, ok := k.Namespaces[ns.Name]
	return ok && stored == ns
}

func (k K8s) dropDetachedMutation(ns *Namespace, status Status) bool {
	return k.NamespacesAccess.LabelSelectorActive && status != DELETED && !k.selectorNamespacePersistent(ns)
}

func (k K8s) GetSecret(namespace, name string) (*Secret, error) {
	ns, ok := k.Namespaces[namespace]
	if !ok || k.configNamespaceMissing(ns) {
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
	if !nsOk || k.configNamespaceMissing(ns) {
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
	if !nsOk || k.configNamespaceMissing(ns) {
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
	return endpoints, err
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

func (k K8s) configNamespaceMissing(ns *Namespace) bool {
	return k.NamespacesAccess.LabelSelectorActive && (ns == nil || !ns.Relevant)
}

// SkipNamespaceInConfig reports whether selector mode must omit this namespace
// from HAProxy generation. Non-selector callers always get false so existing
// Relevant checks keep their original meaning.
func (k K8s) SkipNamespaceInConfig(ns *Namespace) bool {
	if !k.NamespacesAccess.LabelSelectorActive {
		return false
	}
	return ns == nil || !ns.Relevant
}

// SkipIngressInConfig reports whether selector mode must omit this Ingress,
// including when it is reached through another namespace's service index.
func (k K8s) SkipIngressInConfig(ing *Ingress) bool {
	if ing == nil {
		return true
	}
	if ing.Faked {
		return false
	}
	if !k.NamespacesAccess.LabelSelectorActive {
		return false
	}
	ns, ok := k.Namespaces[ing.Namespace]
	return !ok || !ns.Relevant
}

func (k K8s) IsIngressClassSupported(ingressClass, controllerClass string, allowEmptyClass bool) bool {
	var supported bool
	var igClassControllerFromSpec string
	if igClassResource := k.IngressClasses[ingressClass]; igClassResource != nil {
		igClassControllerFromSpec = igClassResource.Controller
	}
	if ingressClass == "" {
		for _, ingressClass := range k.IngressClasses {
			if ingressClass.Annotations["ingressclass.kubernetes.io/is-default-class"] == "true" {
				igClassControllerFromSpec = ingressClass.Controller
				break
			}
		}
	}

	switch controllerClass {
	case "":
		supported = (ingressClass == "" && igClassControllerFromSpec == "") || igClassControllerFromSpec == CONTROLLER
	default:
		supported = ingressClass == "" && allowEmptyClass || igClassControllerFromSpec == filepath.Join(CONTROLLER, controllerClass)
	}

	return supported
}
