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
	"path/filepath"
	"sync"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/route"
	"github.com/haproxytech/kubernetes-ingress/pkg/secret"
	"github.com/haproxytech/kubernetes-ingress/pkg/service"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
)

var ingressClassAnnotationDeprecationOnce sync.Once

type Ingress struct {
	annotations     annotations.Annotations
	resource        *store.Ingress
	controllerClass string
	ruleIDs         []rules.RuleID
	allowEmptyClass bool
	sslPassthrough  bool
}

func logIngressClassAnnotationDeprecationWarning() {
	ingressClassAnnotationDeprecationOnce.Do(func() {
		utils.GetLogger().Warningf("`ingress.class` annotation is deprecated, please use `spec.ingressClassName` instead. Support for `ingress.class` annotation will be removed.")
	})
}

// New returns an Ingress instance to handle the k8s ingress resource given in params.
// If the k8s ingress resource is not assigned to the controller (no matching IngressClass)
// then New will return nil
func New(resource *store.Ingress, class string, emptyClass bool, a annotations.Annotations) *Ingress {
	return &Ingress{resource: resource, controllerClass: class, allowEmptyClass: emptyClass, annotations: a}
}

// Supported verifies if the IngressClass matches the ControllerClass
// and in such case returns true otherwise false
//
// According to https://github.com/kubernetes/api/blob/master/networking/v1/types.go#L257
// ingress.class annotation should have precedence over the IngressClass mechanism implemented
// in "networking.k8s.io".
func (i Ingress) Supported(k8s store.K8s, a annotations.Annotations) (supported bool) {
	if i.resource != nil && i.resource.Faked {
		return true
	}

	var igClassAnn, igClassSpec string
	igClassAnn = a.String("ingress.class", i.resource.Annotations)
	if igClassAnn != "" {
		logIngressClassAnnotationDeprecationWarning()
	}
	if igClassResource := k8s.IngressClasses[i.resource.Class]; igClassResource != nil {
		igClassSpec = igClassResource.Controller
	}
	if igClassAnn == "" && i.resource.Class == "" {
		for _, ingressClass := range k8s.IngressClasses {
			if ingressClass.Annotations["ingressclass.kubernetes.io/is-default-class"] == "true" {
				igClassSpec = ingressClass.Controller
				break
			}
		}
	}

	switch i.controllerClass {
	case "":
		if igClassAnn == "" {
			supported = (i.resource.Class == "" && igClassSpec == "") || igClassSpec == CONTROLLER
		} else if igClassSpec == CONTROLLER {
			logger.Warningf("Ingress '%s/%s': conflicting ingress class mechanisms", i.resource.Namespace, i.resource.Name)
		}
	case igClassAnn:
		supported = true
	default:
		if igClassAnn == "" {
			supported = i.resource.Class == "" && i.allowEmptyClass || igClassSpec == filepath.Join(CONTROLLER, i.controllerClass)
		} else if igClassSpec == filepath.Join(CONTROLLER, i.controllerClass) {
			logger.Warningf("Ingress '%s/%s': conflicting ingress class mechanisms", i.resource.Namespace, i.resource.Name)
		}
	}
	if !supported {
		i.resource.Ignored = true
	}
	if supported && i.resource.Ignored {
		i.resource.Status = store.ADDED
		i.resource.Ignored = false
	}
	return supported
}

func (i *Ingress) handlePath(k store.K8s, h haproxy.HAProxy, host string, path *store.IngressPath, a annotations.Annotations) (err error) {
	svc, err := service.New(k, path, h.Certificates, i.sslPassthrough, i.resource, i.resource.Annotations, k.ConfigMaps.Main.Annotations)
	if err != nil {
		return err
	}
	// Backend
	err = svc.HandleBackend(k, h, a)
	if err != nil {
		return err
	}
	backendName, _ := svc.GetBackendName()
	// If we've got a standalone ingress, put an adhoc RuntimeBackend in HAProxyRuntimeStandalone
	// This RuntimeBackend will be used for runtime update of server lists(enpoints) in EventEndpoints
	if svc.IsStandalone() {
		ns := k.GetNamespace(i.resource.Namespace)
		svcHAProxyRuntimeStandalone := ns.HAProxyRuntimeStandalone[svc.GetResource().Name]
		if svcHAProxyRuntimeStandalone == nil {
			svcHAProxyRuntimeStandalone = map[string]map[string]*store.RuntimeBackend{}
			ns.HAProxyRuntimeStandalone[svc.GetResource().Name] = svcHAProxyRuntimeStandalone
		}
		runtimeBackends := svcHAProxyRuntimeStandalone[path.SvcPortResolved.Name]
		if runtimeBackends == nil {
			runtimeBackends = map[string]*store.RuntimeBackend{}
			svcHAProxyRuntimeStandalone[path.SvcPortResolved.Name] = runtimeBackends
		}
		if runtimeBackends[backendName] == nil {
			runtimeBackends[backendName] = &store.RuntimeBackend{Name: backendName}
		}
	}
	// Route
	ingRoute := route.Route{
		Host:           host,
		Path:           path,
		HAProxyRules:   i.ruleIDs,
		BackendName:    backendName,
		SSLPassthrough: i.sslPassthrough,
	}

	routeACLAnn := a.String("route-acl", svc.GetResource().Annotations)
	if routeACLAnn == "" {
		err = route.AddHostPathRoute(ingRoute, h.Maps)
	} else {
		err = route.AddCustomRoute(ingRoute, routeACLAnn, h)
	}
	if err != nil {
		return err
	}
	// Endpoints
	if _, ok := k.BackendsProcessed[backendName]; !ok {
		svc.HandleHAProxySrvs(k, h)
		k.BackendsProcessed[backendName] = struct{}{}
	}
	return err
}

// HandleAnnotations processes ingress annotations to create HAProxy Rules and constructs
// corresponding list of RuleIDs.
// If Ingress Annotations are at the ConfigMap scope, HAProxy Rules will be applied globally
// without the need to map Rule IDs to specific ingress traffic.
func (i *Ingress) handleAnnotations(k store.K8s, h haproxy.HAProxy) {
	var err error
	result := rules.List{}
	for _, a := range i.annotations.Frontend(i.resource, &result, h.Maps) {
		err = a.Process(k, i.resource.Annotations, k.ConfigMaps.Main.Annotations)
		if err != nil {
			logger.Errorf("Ingress '%s/%s': annotation %s: %s", i.resource.Namespace, i.resource.Name, a.GetName(), err)
		}
	}
	i.ruleIDs = addRules(result, h, true)
}

func HandleCfgMapAnnotations(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) {
	var err error
	result := rules.List{}
	logger.Tracef("Processing Ingress annotations in ConfigMap")
	for _, a := range a.Frontend(nil, &result, h.Maps) {
		err = a.Process(k, k.ConfigMaps.Main.Annotations)
		if err != nil {
			logger.Errorf("ConfigMap: annotation %s: %s", a.GetName(), err)
		}
	}
	addRules(result, h, false)
}

func addRules(list rules.List, h haproxy.HAProxy, ingressRule bool) []rules.RuleID {
	ruleIDs := make([]rules.RuleID, 0, len(list))
	// To avoid inserting twice the same rule id in destinating map file
	ruleIDSet := map[rules.RuleID]struct{}{}
	defaultFrontends := []string{h.FrontHTTP, h.FrontHTTPS}
	for _, rule := range list {
		frontends := defaultFrontends
		switch rule.GetType() {
		case rules.REQ_REDIRECT:
			redirRule := rule.(*rules.RequestRedirect)
			if redirRule.SSLRedirect {
				frontends = []string{h.FrontHTTP}
			} else {
				frontends = []string{h.FrontHTTP, h.FrontHTTPS}
			}
		case rules.REQ_DENY, rules.REQ_CAPTURE:
			if haproxy.SSLPassthrough {
				frontends = []string{h.FrontHTTP, h.FrontSSL}
			}
		}
		for _, frontend := range frontends {
			logger.Error(h.AddRule(frontend, rule, ingressRule || rule.GetType() == rules.REQ_REDIRECT))
			idRule := rules.GetID(rule)
			if _, ok := ruleIDSet[idRule]; !ok {
				ruleIDs = append(ruleIDs, idRule)
				ruleIDSet[idRule] = struct{}{}
			}
		}
	}
	return ruleIDs
}

// Update processes a Kubernetes ingress resource and configures HAProxy accordingly
// by creating corresponding backend, route and HTTP rules.
func (i *Ingress) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) {
	// Default Backend
	if i.resource.DefaultBackend != nil {
		svc, err := service.New(k, i.resource.DefaultBackend, h.Certificates, false, i.resource, i.resource.Annotations, k.ConfigMaps.Main.Annotations)
		if svc != nil {
			err = svc.SetDefaultBackend(k, h, []string{h.FrontHTTP, h.FrontHTTPS}, a)
		}
		if err != nil {
			logger.Errorf("Ingress '%s/%s': default backend: %s", i.resource.Namespace, i.resource.Name, err)
		} else {
			backendName, _ := svc.GetBackendName()
			logger.Infof("Setting http default backend to '%s'", backendName)
		}
	}
	// Ingress secrets
	logger.Tracef("Ingress '%s/%s': processing secrets...", i.resource.Namespace, i.resource.Name)
	secretManager := secret.NewManager(k, h)
	for _, tls := range i.resource.TLS {
		if tls.SecretName == "" {
			continue
		}
		sec := secret.Secret{
			Name:       types.NamespacedName{Namespace: i.resource.Namespace, Name: tls.SecretName},
			SecretType: certs.FT_CERT,
			OwnerType:  secret.OWNERTYPE_INGRESS,
			OwnerName:  i.resource.Name,
		}
		secretManager.Store(sec)
	}
	// Ingress annotations
	if len(i.resource.Rules) == 0 {
		logger.Debugf("Ingress %s/%s: no rules defined", i.resource.Namespace, i.resource.Name)
		return
	}
	logger.Tracef("Ingress '%s/%s': processing annotations...", i.resource.Namespace, i.resource.Name)
	enabled, err := annotations.Bool("ssl-passthrough", i.resource.Annotations, k.ConfigMaps.Main.Annotations)
	if err != nil {
		logger.Error("Ingress '%s/%s': SSL Passthrough parsing: %s", i.resource.Namespace, i.resource.Name, err)
	} else if enabled {
		i.sslPassthrough = true
		haproxy.SSLPassthrough = true
	}
	i.handleAnnotations(k, h)
	// Ingress rules
	logger.Tracef("ingress '%s/%s': processing rules...", i.resource.Namespace, i.resource.Name)
	for _, rule := range i.resource.Rules {
		for _, path := range rule.Paths {
			if err := i.handlePath(k, h, rule.Host, path, a); err != nil {
				logger.Errorf("Ingress '%s/%s': %s", i.resource.Namespace, i.resource.Name, err)
			}
		}
	}
}

func (i Ingress) GetAddresses() []string {
	return i.resource.Addresses
}
