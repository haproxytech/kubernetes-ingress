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
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// setupFrontends creates the HTTP/HTTPS frontends the controller expects to exist,
// committing them so they are visible to the later selection passes.
func setupFrontends(t *testing.T, c *HAProxyController) {
	t.Helper()
	require.NoError(t, c.haproxy.APIStartTransaction())
	require.NoError(t, c.haproxy.FrontendCreate(models.FrontendBase{Name: c.haproxy.FrontHTTP, Mode: "http"}))
	require.NoError(t, c.haproxy.FrontendCreate(models.FrontendBase{Name: c.haproxy.FrontHTTPS, Mode: "http"}))
	require.NoError(t, c.haproxy.APICommitTransaction())
	c.haproxy.APIDisposeTransaction()
	instance.Reset()
}

// addBackendService registers in the store a service and its endpoints so that
// service.New / HandleBackend can build a real HAProxy backend out of it.
func addBackendService(t *testing.T, k store.K8s, ns *store.Namespace, name string) {
	t.Helper()
	k.EventService(ns, &store.Service{
		Namespace: ns.Name,
		Name:      name,
		Status:    store.ADDED,
		Ports: []store.ServicePort{
			{Name: "http", Protocol: "http", Port: 80, Status: store.ADDED},
		},
	})
	k.EventEndpoints(ns, &store.Endpoints{
		Namespace: ns.Name,
		Service:   name,
		SliceName: name,
		Status:    store.ADDED,
		Ports: map[string]*store.PortEndpoints{
			"http": {Port: 80, Addresses: map[string]struct{}{"10.0.0.1": {}}},
		},
	}, nil)
}

// registerDefaultBackendIngress adds to the store an ingress whose spec.defaultBackend
// targets the given service, and returns it.
func registerDefaultBackendIngress(k store.K8s, ns *store.Namespace, name, svcName string) *store.Ingress {
	ing := &store.Ingress{
		IngressCore: store.IngressCore{
			Namespace: ns.Name,
			Name:      name,
			DefaultBackend: &store.IngressPath{
				SvcNamespace:     ns.Name,
				SvcName:          svcName,
				IsDefaultBackend: true,
			},
		},
		Status: store.ADDED,
	}
	ns.Ingresses[name] = ing
	return ing
}

func applyDefaultBackendPass(t *testing.T, c *HAProxyController, candidates ...*store.Ingress) string {
	t.Helper()
	require.NoError(t, c.haproxy.APIStartTransaction())
	c.defaultBackend = nil
	for _, ing := range candidates {
		c.considerDefaultBackend(ing)
	}
	c.setIngressDefaultBackend()
	require.NoError(t, c.haproxy.APICommitTransaction())
	c.haproxy.APIDisposeTransaction()

	front, err := c.haproxy.FrontendGet(c.haproxy.FrontHTTP)
	require.NoError(t, err)
	return front.DefaultBackend
}

// TestSetIngressDefaultBackendDeterministic checks that when several ingresses declare
// a spec.defaultBackend, the frontend default backend is always the same one (the
// ingress with the smallest key) no matter the order candidates are submitted in. This
// is the order in which processIngress() walks the Go maps, which is random.
func TestSetIngressDefaultBackendDeterministic(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "svc-a")
	addBackendService(t, c.store, ns, "svc-b")

	ingA := registerDefaultBackendIngress(c.store, ns, "ingress-a", "svc-a")
	ingB := registerDefaultBackendIngress(c.store, ns, "ingress-b", "svc-b")

	// ingress-a has the smallest key, so its backend (built from svc-a) must win.
	expected := "ns_svc_svc-a_http"

	require.Equal(t, expected, applyDefaultBackendPass(t, c, ingA, ingB),
		"default backend must be the winner when submitted in order")
	instance.Reset()
	require.Equal(t, expected, applyDefaultBackendPass(t, c, ingB, ingA),
		"default backend must be the same winner when submitted in reverse order")
}

// TestSetIngressDefaultBackendIdempotent checks that re-running the selection with an
// unchanged set of ingresses does not request an HAProxy reload: the previous code
// flipped the default backend between candidates and reloaded on every sync.
func TestSetIngressDefaultBackendIdempotent(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "svc-a")
	addBackendService(t, c.store, ns, "svc-b")

	ingA := registerDefaultBackendIngress(c.store, ns, "ingress-a", "svc-a")
	ingB := registerDefaultBackendIngress(c.store, ns, "ingress-b", "svc-b")

	// First pass legitimately reloads (creates backends, sets the default backend).
	applyDefaultBackendPass(t, c, ingA, ingB)
	instance.Reset()

	// Steady state: identical inputs, submitted in a different order, must not reload.
	applyDefaultBackendPass(t, c, ingB, ingA)
	require.False(t, instance.NeedReload(),
		"steady-state default backend selection must not request a reload")
}
