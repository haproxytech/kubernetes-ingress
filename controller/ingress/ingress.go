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
	"path/filepath"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/route"
	"github.com/haproxytech/kubernetes-ingress/controller/service"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Ingress struct {
	resource        *store.Ingress
	ruleIDs         []rules.RuleID
	controllerClass string
	allowEmptyClass bool
	sslPassthrough  bool
}

// New returns an Ingress instance to handle the k8s ingress resource given in params.
// If the k8s ingress resource is not assigned to the controller (no matching IngressClass)
// then New will return nil
func New(k store.K8s, resource *store.Ingress, class string, emptyClass bool) *Ingress {
	i := &Ingress{resource: resource, controllerClass: class, allowEmptyClass: emptyClass}
	if i.resource == nil || !i.supported(k) {
		return nil
	}
	return i
}

// supported verifies if the IngressClass matches the ControllerClass
// and in such case returns true otherwise false
//
// According to https://github.com/kubernetes/api/blob/master/networking/v1/types.go#L257
// ingress.class annotation should have precedence over the IngressClass mechanism implemented
// in "networking.k8s.io".
func (i Ingress) supported(k8s store.K8s) (supported bool) {
	var igClassAnn, igClassSpec string
	igClassAnn = annotations.String("ingress.class", i.resource.Annotations)
	if igClassResource := k8s.IngressClasses[i.resource.Class]; igClassResource != nil && igClassResource.Status != store.DELETED {
		igClassSpec = igClassResource.Controller
	}

	defer func() {
		if supported && i.resource.Ignored {
			i.resource.Status = store.ADDED
			i.resource.Ignored = false
		}
	}()

	if i.controllerClass == "" {
		if igClassAnn == "" && igClassSpec == "" {
			supported = true
			return
		}
		if igClassSpec == CONTROLLER {
			supported = true
			return
		}
	} else {
		if igClassAnn == "" && igClassSpec == "" && i.allowEmptyClass {
			supported = true
			return
		}
		if igClassAnn == i.controllerClass {
			supported = true
			return
		}
		if igClassSpec == filepath.Join(CONTROLLER, i.controllerClass) {
			supported = true
			return
		}
	}
	i.resource.Ignored = true
	return
}

func (i *Ingress) handlePath(k store.K8s, cfg *configuration.ControllerCfg, api api.HAProxyClient, host string, path *store.IngressPath) (reload bool, err error) {
	svc, err := service.New(k, path, cfg.Certificates, i.sslPassthrough, i.resource.Annotations, k.ConfigMaps.Main.Annotations)
	if err != nil {
		return
	}
	if path.Status == store.DELETED {
		return
	}
	// Backend
	backendReload, err := svc.HandleBackend(api, k)
	if err != nil {
		return
	}
	backendName, _ := svc.GetBackendName()
	// Route
	var routeReload bool
	ingRoute := route.Route{
		Host:           host,
		Path:           path,
		HAProxyRules:   i.ruleIDs,
		BackendName:    backendName,
		SSLPassthrough: i.sslPassthrough,
	}

	routeACLAnn := annotations.String("route-acl", svc.GetResource().Annotations)
	if routeACLAnn == "" {
		if _, ok := route.CustomRoutes[backendName]; ok {
			delete(route.CustomRoutes, backendName)
			logger.Debugf("Custom Route to backend '%s' deleted, reload required", backendName)
			routeReload = true
		}
		err = route.AddHostPathRoute(ingRoute, cfg.MapFiles)
	} else {
		routeReload, err = route.AddCustomRoute(ingRoute, routeACLAnn, api)
	}
	if err != nil {
		return
	}
	cfg.ActiveBackends[backendName] = struct{}{}
	// Endpoints
	endpointsReload := svc.HandleHAProxySrvs(api, k)
	return backendReload || endpointsReload || routeReload, err
}

// HandleAnnotations processes ingress annotations to create HAProxy Rules and constructs
// corresponding list of RuleIDs.
// If Ingress Annotations are at the ConfigMap scope, HAProxy Rules will be applied globally
// without the need to map Rule IDs to specific ingress traffic.
func (i *Ingress) HandleAnnotations(k store.K8s, cfg *configuration.ControllerCfg) {
	var err error
	var ingressRule bool
	var annSource string
	var annList map[string]string
	var result = rules.Rules{}
	if i.resource == nil {
		logger.Tracef("Processing Ingress annotations in ConfigMap")
		annSource = "ConfigMap"
		annList = k.ConfigMaps.Main.Annotations
		ingressRule = false
	} else {
		annSource = fmt.Sprintf("Ingress '%s/%s'", i.resource.Namespace, i.resource.Name)
		annList = i.resource.Annotations
		ingressRule = true
	}
	frontends := []string{cfg.FrontHTTP, cfg.FrontHTTPS}
	for _, a := range annotations.Frontend(i.resource, &result, *cfg.MapFiles) {
		err = a.Process(k, annList)
		if err != nil {
			logger.Errorf("%s: annotation %s: %s", annSource, a.GetName(), err)
		}
	}
	for _, rule := range result {
		switch rule.GetType() {
		case rules.REQ_REDIRECT:
			redirRule := rule.(*rules.RequestRedirect)
			if redirRule.SSLRedirect {
				frontends = []string{cfg.FrontHTTP}
			} else {
				frontends = []string{cfg.FrontHTTP, cfg.FrontHTTPS}
			}
		case rules.REQ_DENY, rules.REQ_CAPTURE:
			if i.sslPassthrough {
				frontends = []string{cfg.FrontHTTP, cfg.FrontSSL}
			}
		case rules.REQ_RATELIMIT:
			limitRule := rule.(*rules.ReqRateLimit)
			cfg.RateLimitTables = append(cfg.RateLimitTables, limitRule.TableName)
		}
		for _, frontend := range frontends {
			logger.Error(cfg.HAProxyRules.AddRule(rule, ingressRule, frontend))
		}
		i.ruleIDs = append(i.ruleIDs, rules.GetID(rule))
	}
}

// Update processes a Kubernetes ingress resource and configures HAProxy accordingly
// by creating corresponding backend, route and HTTP rules.
func (i *Ingress) Update(k store.K8s, cfg *configuration.ControllerCfg, api api.HAProxyClient) (reload bool) {
	// Default Backend
	if i.resource.DefaultBackend != nil {
		svc, err := service.New(k, i.resource.DefaultBackend, cfg.Certificates, false, i.resource.Annotations, k.ConfigMaps.Main.Annotations)
		if svc != nil {
			reload, err = svc.SetDefaultBackend(k, cfg, api, []string{cfg.FrontHTTP, cfg.FrontHTTPS})
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
	for _, tls := range i.resource.TLS {
		if tls.Status == store.DELETED {
			continue
		}
		secret, secErr := k.GetSecret(i.resource.Namespace, tls.SecretName)
		if secErr != nil {
			logger.Warningf("Ingress '%s/%s': %s", i.resource.Namespace, i.resource.Name, secErr)
			continue
		}
		_, err := cfg.Certificates.HandleTLSSecret(secret, certs.FT_CERT)
		logger.Error(err)
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
		cfg.SSLPassthrough = true
	}
	i.HandleAnnotations(k, cfg)
	// Ingress rules
	logger.Tracef("ingress '%s/%s': processing rules...", i.resource.Namespace, i.resource.Name)
	for _, rule := range i.resource.Rules {
		for _, path := range rule.Paths {
			if r, err := i.handlePath(k, cfg, api, rule.Host, path); err != nil {
				logger.Errorf("Ingress '%s/%s': %s", i.resource.Namespace, i.resource.Name, err)
			} else {
				reload = reload || r
			}
		}
	}
	return
}
