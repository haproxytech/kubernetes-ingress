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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/fs"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// registerSharedIngress adds an ingress on its own host, pointing at a shared service
// port, and optionally asking for ssl-passthrough.
func registerSharedIngress(ns *store.Namespace, name, host, svcName string, passthrough bool, age time.Time) *store.Ingress {
	annotations := map[string]string{}
	if passthrough {
		annotations["ssl-passthrough"] = "true"
	}
	ing := &store.Ingress{
		IngressCore: store.IngressCore{
			Namespace:   ns.Name,
			Name:        name,
			Annotations: annotations,
			Rules: map[string]*store.IngressRule{
				host: {
					Host: host,
					Paths: map[string]*store.IngressPath{
						"/": {
							Path:          "/",
							PathTypeMatch: store.PATH_TYPE_PREFIX,
							SvcNamespace:  ns.Name,
							SvcName:       svcName,
							SvcPortString: "http",
						},
					},
				},
			},
		},
		Status: store.ADDED,
	}
	ing.CreationTime = age
	ns.Ingresses[name] = ing
	return ing
}

// processIngresses runs one reconciliation pass over the ingresses of the store, the way
// processIngressesDefaultImplementation does, and returns the mode of the backend.
func processIngresses(t *testing.T, c *HAProxyController, backendName string) string {
	t.Helper()
	require.NoError(t, c.haproxy.APIStartTransaction())
	c.store.BackendsProcessed = map[string]store.BackendOwner{}
	for _, namespace := range sortedByKey(c.store.Namespaces) {
		for _, ing := range sortedIngresses(namespace.Ingresses) {
			ingress.New(ing, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, c.annotations).
				Update(c.store, c.haproxy, c.annotations)
		}
	}
	require.NoError(t, c.haproxy.APICommitTransaction())
	c.haproxy.APIDisposeTransaction()

	backend, err := c.haproxy.BackendGet(backendName)
	require.NoError(t, err, "the shared backend must exist")
	return backend.Mode
}

// mapContent flushes the map files and returns the content of one of them. Maps are
// updated through the runtime socket and written to disk lazily, so the write has to be
// forced before reading.
func mapContent(t *testing.T, c *HAProxyController, name string) string {
	t.Helper()
	c.haproxy.RefreshMaps(c.haproxy.HAProxyClient)
	fs.RunDelayedFuncs()
	fs.Writer.WaitUntilWritesDone()
	content, err := os.ReadFile(filepath.Join(c.haproxy.Env.MapsDir, name+".map"))
	require.NoError(t, err)
	return string(content)
}

// TestEveryPathOfTheOwnerIsRouted pins what the self check in the ownership branch is
// for. handlePath runs once per path, so an ingress declaring several paths on the same
// service port claims the same backend several times and finds it, from the second path
// on, already owned - by itself. Every one of its paths must still be routed: only an
// ingress disagreeing with *another* one on the mode loses its route.
func TestEveryPathOfTheOwnerIsRouted(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "shared")

	ing := registerSharedIngress(ns, "one-ing", "h.local", "shared", false,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	ing.Rules["h.local"].Paths["/second"] = &store.IngressPath{
		Path: "/second", PathTypeMatch: store.PATH_TYPE_PREFIX,
		SvcNamespace: ns.Name, SvcName: "shared", SvcPortString: "http",
	}

	require.Equal(t, "http", processIngresses(t, c, "ns_svc_shared_http"))

	paths := mapContent(t, c, "path-prefix")
	require.Contains(t, paths, "h.local/	", "the first path must be routed")
	require.Contains(t, paths, "h.local/second/	", "the second path must be routed too")
}

// TestSharedBackendKeepsOwnerModeWhateverTheNewcomerName is the regression test for a
// backend being taken over by an ingress created afterwards.
//
// A backend name derives from (namespace, service, port name), so two ingresses on the
// same service port share one backend, whose definition is rebuilt from scratch out of
// the annotations of the ingress being processed and replaces the previous one. The
// backend therefore used to be configured by whichever ingress came last: adding one
// which asks for ssl-passthrough was enough to flip an established backend to mode tcp,
// breaking every layer 7 route already pointing at it, and reloading haproxy.
//
// The oldest ingress now owns the backend. The newcomer here is deliberately named to
// sort *before* the established one, which is what ordering by name alone would not
// protect against.
func TestSharedBackendKeepsOwnerModeWhateverTheNewcomerName(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "shared")
	const backendName = "ns_svc_shared_http"

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	registerSharedIngress(ns, "z-established", "z.local", "shared", false, old)
	require.Equal(t, "http", processIngresses(t, c, backendName))

	// Created later, but sorting first by name.
	registerSharedIngress(ns, "a-newcomer", "a.local", "shared", true, old.Add(time.Hour))
	require.Equal(t, "http", processIngresses(t, c, backendName),
		"an established backend must keep the mode of the ingress which constituted it, "+
			"whatever the name of a later ingress")
}

// TestSharedBackendOwnedByOldest pins the ordering key: age decides, not the name. The
// oldest ingress asks for ssl-passthrough and is named last, so a tcp backend proves the
// creation time won over the lexicographic order.
func TestSharedBackendOwnedByOldest(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "shared")
	const backendName = "ns_svc_shared_http"

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	registerSharedIngress(ns, "z-oldest", "z.local", "shared", true, old)
	registerSharedIngress(ns, "a-newest", "a.local", "shared", false, old.Add(time.Hour))

	require.Equal(t, "tcp", processIngresses(t, c, backendName),
		"the backend must be built from the oldest ingress, not from the first by name")
}

// TestSharedBackendFallsBackToNameOnEqualAge covers the common tie: Kubernetes stores
// creationTimestamp with second granularity, so ingresses applied together carry the
// same one and the name has to settle it.
func TestSharedBackendFallsBackToNameOnEqualAge(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "shared")
	const backendName = "ns_svc_shared_http"

	same := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	registerSharedIngress(ns, "a-ing", "a.local", "shared", true, same)
	registerSharedIngress(ns, "b-ing", "b.local", "shared", false, same)

	require.Equal(t, "tcp", processIngresses(t, c, backendName),
		"on equal creation time the first by name must own the backend")
}

// TestSharedBackendRefusesRouteOnModeConflict is the other half of ownership: refusing to
// rebuild the definition is not enough, the route must be refused too.
//
// A passthrough ingress writes its host into sni.map, which makes the ssl frontend switch
// straight to the backend. Pointed at a backend another ingress built in http mode, that
// entry sends raw TLS bytes to an http-mode backend - and if both ingresses serve the
// same host, it is the owner's traffic that breaks, since the entry short-circuits the
// offload path the owner relies on. No sni entry must therefore be produced for the
// ingress which lost the mode.
func TestSharedBackendRefusesRouteOnModeConflict(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "shared")
	const backendName = "ns_svc_shared_http"

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	registerSharedIngress(ns, "z-established", "shared.local", "shared", false, old)
	registerSharedIngress(ns, "a-newcomer", "shared.local", "shared", true, old.Add(time.Hour))

	require.Equal(t, "http", processIngresses(t, c, backendName))
	require.NotContains(t, mapContent(t, c, "sni"), "shared.local",
		"the ingress which lost the mode must not put its host in sni.map, or the ssl frontend "+
			"would route the owner's traffic to a backend in the wrong mode")
}

// TestSharedBackendKeepsRouteWhenModesAgree pins the negative: losing the backend tuning
// is not a reason to stop serving an ingress. Sharing a backend is legitimate, the
// servers are the same, and the route must be created.
func TestSharedBackendKeepsRouteWhenModesAgree(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "shared")
	const backendName = "ns_svc_shared_http"

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	registerSharedIngress(ns, "z-established", "z.local", "shared", true, old)
	registerSharedIngress(ns, "a-newcomer", "a.local", "shared", true, old.Add(time.Hour))

	require.Equal(t, "tcp", processIngresses(t, c, backendName))
	// Asserted on the content and not merely on the map being non-empty: the owner alone
	// would fill it, so an over-broad refusal would go unnoticed.
	sni := mapContent(t, c, "sni")
	require.Contains(t, sni, "z.local", "the owner must be routed")
	require.Contains(t, sni, "a.local",
		"both ingresses agree on the mode, so the one which does not own the backend must be routed too")
}
