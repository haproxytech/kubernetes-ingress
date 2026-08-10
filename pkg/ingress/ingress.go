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
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/route"
	"github.com/haproxytech/kubernetes-ingress/pkg/secret"
	"github.com/haproxytech/kubernetes-ingress/pkg/service"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"k8s.io/apimachinery/pkg/types"
)

type Ingress struct {
	annotations     annotations.Annotations
	resource        *store.Ingress
	controllerClass string
	ruleIDs         []rules.RuleID
	allowEmptyClass bool
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

	supported = k8s.IsIngressClassSupported(i.resource.Class, i.controllerClass, i.allowEmptyClass)
	if !supported {
		i.resource.Ignored = true
	}
	if supported && i.resource.Ignored {
		i.resource.Status = store.ADDED
		i.resource.Ignored = false
	}
	return supported
}

// handlePath configures the backend, the servers and the route of one ingress path, and
// reports whether a route was created for it - which is not the same as the absence of an
// error: a path whose backend was constituted by another ingress in the other mode is
// deliberately left unrouted.
func (i *Ingress) handlePath(k store.K8s, h haproxy.HAProxy, host string, path *store.IngressPath, a annotations.Annotations) (routed bool, err error) {
	// Resolved here rather than once for the whole ingress: the annotation describes the
	// service behind the path, so each path gets the mode of its own service.
	sslPassthrough, err := SSLPassthroughEnabled(k, path, i.resource.Annotations)
	if err != nil {
		logger.Errorf("Ingress '%s/%s': SSL Passthrough parsing for service '%s/%s': %s",
			i.resource.Namespace, i.resource.Name, path.SvcNamespace, path.SvcName, err)
	}
	svc, err := service.New(k, path, h.Certificates, sslPassthrough, i.resource, i.resource.Annotations, k.ConfigMaps.Main.Annotations)
	if err != nil {
		return false, err
	}
	backendName, err := svc.GetBackendName()
	if err != nil {
		return false, err
	}
	// Backend. The first ingress to reference a backend constitutes it and owns it
	// entirely: mode, balance, options, checks, config snippets - everything
	// getBackendModel builds. Later ingresses referencing the same service port get their
	// route below and share the servers, but do not rebuild the definition, which they
	// would replace wholesale rather than merge into. Ownership by the first one rather
	// than by the last is what keeps an established backend from being taken over, and
	// reconfigured and reloaded, by an ingress created afterwards.
	if owner, owned := k.BackendsProcessed[backendName]; !owned {
		if err = svc.HandleBackend(k, h, a); err != nil {
			return false, err
		}
		k.BackendsProcessed[backendName] = store.BackendOwner{Ingress: i.fqn(), Passthrough: sslPassthrough}
		svc.HandleHAProxySrvs(k, h)
	} else if owner.Ingress != i.fqn() && !i.servableWithBackendOwnedByOther(owner, backendName, sslPassthrough) {
		return false, nil
	}
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
		SSLPassthrough: sslPassthrough,
	}

	routeACLAnn := a.String("route-acl", svc.GetResource().Annotations)
	if routeACLAnn == "" {
		i.reportRouteKeyCollisions(k, ingRoute)
		err = route.AddHostPathRoute(ingRoute, h.Maps)
	} else {
		err = route.AddCustomRoute(ingRoute, routeACLAnn, h)
	}
	return err == nil, err
}

// backendModeConflict is logged when an ingress cannot be served through the backend it
// references, because another one constituted it in the other mode. Split over several
// lines because revive caps source lines at 200 characters.
const backendModeConflict = "backend '%s' is built from ingress '%s' with ssl-passthrough=%t, the first ingress to " +
	"reference it, so ingress '%s' which asked for ssl-passthrough=%t is not routed to it at all: a backend has a " +
	"single mode, and routing to one in the wrong mode would break the traffic of both ingresses. Set the " +
	"'standalone-backend' annotation on it to give it a dedicated backend"

// backendAnnotationsDropped is logged when an ingress agrees with the owner on the mode,
// so it is served, but carries backend annotations of its own which the owner's definition
// leaves unapplied. Naming them is the point: the effect is otherwise invisible.
const backendAnnotationsDropped = "backend '%s' was constituted by ingress '%s', so the backend annotations declared " +
	"by ingress '%s' are not applied: %s. Move them to the service, which both ingresses share and which takes " +
	"precedence over an ingress, or set the 'standalone-backend' annotation to get a dedicated backend"

// servableWithBackendOwnedByOther reports that the backend was constituted by another
// ingress, and returns whether this ingress can still be served through it.
//
// It cannot when the two disagree on the mode. Its route must then not be created
// either: a route to a backend in the wrong mode does not merely break this ingress, it
// can break the owner as well. Two ingresses sharing a host is enough - the sni map
// entry of a passthrough ingress makes the ssl frontend switch straight to the backend,
// short-circuiting the offload path the owner relies on, so raw TLS bytes reach a
// backend in http mode. Refusing the route keeps that host on the offload path, where it
// works.
//
// A disagreement is therefore a warning. Anything else is a debug message: sharing a
// backend is legitimate and common, the servers are the same ones since the service port
// is the same, and only the tuning of the owner applies - which is now a deterministic
// outcome rather than a surprise.
// The mode is the one resolved for the path being handled, not a property of the ingress:
// since ssl-passthrough is resolved per path against its own service, one ingress can ask for
// both modes on two different services.
func (i *Ingress) servableWithBackendOwnedByOther(owner store.BackendOwner, backendName string, sslPassthrough bool) bool {
	if owner.Passthrough != sslPassthrough {
		logger.Warningf(backendModeConflict, backendName, owner.Ingress, owner.Passthrough, i.fqn(), sslPassthrough)
		return false
	}
	if dropped := i.declaredBackendAnnotations(); len(dropped) > 0 {
		logger.Warningf(backendAnnotationsDropped, backendName, owner.Ingress, i.fqn(), strings.Join(dropped, ", "))
		return true
	}
	logger.Debugf("backend '%s' was constituted by ingress '%s'; ingress '%s' declares no backend annotation of its own",
		backendName, owner.Ingress, i.fqn())
	return true
}

// declaredBackendAnnotations returns the backend annotations this ingress carries itself,
// in registry order, or nothing when it carries none.
//
// Only the annotations of the ingress are looked at. Those of the service are shared with
// every ingress referencing it and take precedence over the ingress ones in service.New,
// so they are never what an ingress loses to the owner of a backend.
func (i *Ingress) declaredBackendAnnotations() []string {
	declared := make([]string, 0, 2)
	for _, name := range annotations.BackendNames() {
		if _, ok := i.resource.Annotations[name]; ok {
			declared = append(declared, name)
		}
	}
	return declared
}

// resolveFrontendRules processes the frontend annotations of the ingress and returns the
// rules they ask for, recording their ids so the routes can carry them.
//
// The rules are returned undeclared on purpose. An ingress-scoped rule only applies to the
// traffic whose route carries its id in the map value it resolves to, so declaring a rule
// for an ingress which ends up with no route at all puts a condition in the configuration
// that no map value can ever satisfy. rules.GetID being a pure hash of the rule content,
// the ids are known before the declaration, which is what lets Update wait until a route
// exists.
func (i *Ingress) resolveFrontendRules(k store.K8s, h haproxy.HAProxy) rules.List {
	result := rules.List{}
	for _, a := range i.annotations.Frontend(i.resource, &result, h.Maps) {
		if err := a.Process(k, i.resource.Annotations, k.ConfigMaps.Main.Annotations); err != nil {
			logger.Errorf("Ingress '%s/%s': annotation %s: %s", i.resource.Namespace, i.resource.Name, a.GetName(), err)
		}
	}
	i.ruleIDs = ruleIDs(result)
	return result
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
	declareRules(result, h, false)
}

// ruleIDs returns the ids of the rules of list, in order and without duplicates: the same
// rule is attached to several frontends, and its id must appear once in a map value.
func ruleIDs(list rules.List) []rules.RuleID {
	ids := make([]rules.RuleID, 0, len(list))
	seen := map[rules.RuleID]struct{}{}
	for _, rule := range list {
		id := rules.GetID(rule)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

// declareRules attaches each rule to the frontends which have to carry it.
func declareRules(list rules.List, h haproxy.HAProxy, ingressRule bool) {
	defaultFrontends := []string{h.FrontHTTP, h.FrontHTTPS}
	for _, rule := range list {
		frontends := defaultFrontends
		switch rule.GetType() {
		case rules.REQ_REDIRECT:
			//revive:disable-next-line:unchecked-type-assertion
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
		}
	}
}

// Update processes a Kubernetes ingress resource and configures HAProxy accordingly
// by creating corresponding backend, route and HTTP rules.
func (i *Ingress) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) {
	// The ingress spec.defaultBackend is NOT handled here. The HTTP/HTTPS frontends
	// have a single shared default backend, so letting every ingress apply its own
	// would make the result depend on the (random) order in which ingresses are
	// iterated and trigger spurious reloads. Selection is centralized and made
	// deterministic in HAProxyController.setIngressDefaultBackend.
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
	frontendRules := i.resolveFrontendRules(k, h)
	// Ingress rules
	logger.Tracef("ingress '%s/%s': processing rules...", i.resource.Namespace, i.resource.Name)
	routed := false
	for _, rule := range i.resource.Rules {
		for _, path := range rule.Paths {
			pathRouted, err := i.handlePath(k, h, rule.Host, path, a)
			if err != nil {
				logger.Errorf("Ingress '%s/%s': %s", i.resource.Namespace, i.resource.Name, err)
			}
			routed = routed || pathRouted
		}
	}
	// The frontend rules are declared last, and only once a route carries their ids. An
	// ingress-scoped rule tests its own id against the map value the request resolves to,
	// so an ingress left with no route at all would otherwise put rules in the generated
	// configuration that no map value can satisfy: they can never match, yet they read as
	// configured. Declaring them here rather than before the paths does not change their
	// order in a frontend, which stays the order the ingresses are walked in.
	if routed {
		declareRules(frontendRules, h, true)
	} else if len(frontendRules) > 0 {
		logger.Debugf("Ingress '%s/%s': no route was created, so its %d frontend rule(s) are not declared",
			i.resource.Namespace, i.resource.Name, len(frontendRules))
	}
}

func (i Ingress) GetAddresses() []string {
	return i.resource.Addresses
}
