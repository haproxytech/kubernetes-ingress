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
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/service"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// considerDefaultBackend registers an ingress declaring a spec.defaultBackend as a
// candidate for the frontends' shared default backend.
//
// The HTTP/HTTPS frontends expose a single default backend. Kubernetes leaves the
// outcome undefined when several ingresses set a spec.defaultBackend, and the
// controller iterates ingresses in (random) Go map order, so applying each one in
// turn made the winner depend on iteration order: the resulting config was
// non-deterministic and flapped between reconciliations, triggering spurious HAProxy
// reloads. Instead we collect the candidates and let pickDefaultBackend select a
// stable winner, applied once in setIngressDefaultBackend.
//
// The candidate is resolved to its canonical store ingress so the default backend is
// always built from the ingress' own annotations, never a per-service merged copy
// produced by processIngressesWithMerge (which would reintroduce non-determinism).
func (c *HAProxyController) considerDefaultBackend(ing *store.Ingress) {
	if ing == nil || ing.DefaultBackend == nil {
		return
	}
	canonical := ing
	if ns, ok := c.store.Namespaces[ing.Namespace]; ok {
		if stored := ns.Ingresses[ing.Name]; stored != nil {
			canonical = stored
		}
	}
	c.defaultBackend = pickDefaultBackend(c.defaultBackend, canonical)
}

// pickDefaultBackend returns, between the current winner and a new candidate, the
// ingress with the smallest (namespace, name) key. This tie-break makes the
// selection independent of the order in which candidates are submitted, which is the
// property that removes the non-determinism.
func pickDefaultBackend(current, candidate *store.Ingress) *store.Ingress {
	if candidate == nil {
		return current
	}
	if current == nil {
		return candidate
	}
	if defaultBackendKey(candidate) < defaultBackendKey(current) {
		return candidate
	}
	return current
}

func defaultBackendKey(ing *store.Ingress) string {
	return ing.Namespace + "/" + ing.Name
}

// setIngressDefaultBackend applies the default backend of the deterministically
// selected ingress (if any) to the HTTP/HTTPS frontends. It runs after
// processIngress, so the ingress default backend keeps overriding the global one set
// by handleGlobalConfig, but is now applied exactly once per reconciliation.
func (c *HAProxyController) setIngressDefaultBackend() {
	if c.defaultBackend == nil {
		return
	}
	ing := c.defaultBackend
	svc, err := service.New(c.store, ing.DefaultBackend, c.haproxy.Certificates, false, ing, ing.Annotations, c.store.ConfigMaps.Main.Annotations)
	if err == nil {
		c.reportPassthroughOnDefaultBackend(ing, svc)
		err = svc.SetDefaultBackend(c.store, c.haproxy, []string{c.haproxy.FrontHTTP, c.haproxy.FrontHTTPS}, c.annotations)
	}
	if err != nil {
		logger.Errorf("Ingress '%s/%s': default backend: %s", ing.Namespace, ing.Name, err)
		return
	}
	backendName, _ := svc.GetBackendName()
	logger.Infof("Setting http default backend to '%s'", backendName)
}

// defaultBackendPassthroughIgnored is logged when the service of a spec.defaultBackend asks
// to be served in ssl-passthrough. Split over several lines because revive caps source lines
// at 200 characters.
const defaultBackendPassthroughIgnored = "ingress '%s/%s': ssl-passthrough is declared for the default backend " +
	"service '%s/%s' but cannot apply to it: a default backend serves the requests matching no host, so no sni entry " +
	"can route to it, and it takes the mode of the http and https frontends it is attached to. Declare a rule for the " +
	"host to be served in passthrough, or expose the service on its own port through the tcp-services ConfigMap or a " +
	"TCP custom resource, which is how a service answering every connection on a port in tcp mode is configured"

// reportPassthroughOnDefaultBackend warns that ssl-passthrough does not reach a default
// backend, rather than letting the annotation be a silent no-op.
//
// SetDefaultBackend takes the mode from the frontend it attaches the backend to, so a
// default backend on the http and https frontends is in http mode whatever its service
// asks for. That rule is a guard, not an oversight: a static default_backend from an http
// frontend to a tcp backend is an haproxy ALERT ("tries to use incompatible tcp backend"),
// which refuses the whole transaction, so honouring the annotation here would wedge the
// controller rather than serve anything.
//
// Nothing else could be done with it either. Passthrough routing is keyed on sni.map, which
// only carries host entries, and a default backend is by definition what serves the requests
// no host matched. A true tls catch-all would have to be the default backend of the ssl
// frontend, which is ssl-backend - the only path from there to the https frontend - so
// repointing it would break every layer 7 https route of the controller. This is also why
// the pass deciding the layer 4 topology does not consult a default backend.
//
// A service answering every connection arriving on a port in tcp mode is a supported
// configuration, through the tcp-services ConfigMap or a TCP custom resource: both build
// their own tcp frontend and call SetDefaultBackend on it, where the same mode-from-frontend
// rule yields tcp. That is what the message points at.
//
// The configmap is deliberately left out of the resolution: a cluster-wide default is not a
// mistake worth reporting on every sync of every deployment which sets it. An annotation put
// on this service or on this ingress is.
func (c *HAProxyController) reportPassthroughOnDefaultBackend(ing *store.Ingress, svc *service.Service) {
	declared, err := annotations.Bool("ssl-passthrough", svc.GetResource().Annotations, ing.Annotations)
	if err != nil || !declared {
		return
	}
	logger.Warningf(defaultBackendPassthroughIgnored, ing.Namespace, ing.Name,
		svc.GetResource().Namespace, svc.GetResource().Name)
}
