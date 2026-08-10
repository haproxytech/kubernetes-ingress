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

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/fs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// passthroughService registers a service with its endpoints and puts the annotations on
// the service resource itself, which is what the ssl-passthrough resolution now reads
// first.
func passthroughService(t *testing.T, c *HAProxyController, ns *store.Namespace, name string, annotations map[string]string) {
	t.Helper()
	addBackendService(t, c.store, ns, name)
	ns.Services[name].Annotations = annotations
}

// registerRoutedIngress adds an ingress routing each host of hostToSvc to the http port
// of the corresponding service.
func registerRoutedIngress(ns *store.Namespace, name string, annotations, hostToSvc map[string]string) *store.Ingress {
	rules := map[string]*store.IngressRule{}
	for host, svcName := range hostToSvc {
		rules[host] = &store.IngressRule{
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
		}
	}
	ing := &store.Ingress{
		IngressCore: store.IngressCore{
			Namespace:   ns.Name,
			Name:        name,
			Annotations: annotations,
			Rules:       rules,
		},
		Status: store.ADDED,
	}
	ns.Ingresses[name] = ing
	return ing
}

// syncIngresses runs the two passes of a reconciliation which matter here: deciding
// whether the passthrough topology is on, then processing the ingresses. The global flag
// is reset afterwards, being package state shared by every test of this binary.
func syncIngresses(t *testing.T, c *HAProxyController) {
	t.Helper()
	t.Cleanup(func() { haproxy.SSLPassthrough = false })

	c.processSSLPassthroughInConfigFile()
	require.NoError(t, c.haproxy.APIStartTransaction())
	c.store.BackendsProcessed = map[string]store.BackendOwner{}
	for _, namespace := range c.store.Namespaces {
		for _, ing := range namespace.Ingresses {
			ingress.New(ing, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, c.annotations).
				Update(c.store, c.haproxy, c.annotations)
		}
	}
	require.NoError(t, c.haproxy.APICommitTransaction())
	c.haproxy.APIDisposeTransaction()
}

// backendMode returns the mode HAProxy ended up with for a service port.
func backendMode(t *testing.T, c *HAProxyController, svcName string) string {
	t.Helper()
	backend, err := c.haproxy.BackendGet("ns_svc_" + svcName + "_http")
	require.NoError(t, err, "the backend of service '%s' must exist", svcName)
	return backend.Mode
}

// mapFileContent flushes the map files and returns the content of one of them. Maps are
// updated through the runtime socket and written to disk lazily, so the write has to be
// forced before reading.
func mapFileContent(t *testing.T, c *HAProxyController, name string) string {
	t.Helper()
	c.haproxy.RefreshMaps(c.haproxy.HAProxyClient)
	fs.RunDelayedFuncs()
	fs.Writer.WaitUntilWritesDone()
	content, err := os.ReadFile(filepath.Join(c.haproxy.Env.MapsDir, name+".map"))
	require.NoError(t, err)
	return string(content)
}

// TestServiceAnnotationEnablesPassthrough is the headline case: the annotation is
// declared on the service, the ingress carries nothing, and the traffic must still be
// served at layer 4. Both halves are asserted, because either one alone is a broken
// configuration: the backend must be in tcp mode, and the host must be in sni.map, which
// is the only index the ssl frontend routes on.
func TestServiceAnnotationEnablesPassthrough(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	passthroughService(t, c, ns, "tls-svc", map[string]string{"ssl-passthrough": "true"})
	registerRoutedIngress(ns, "ing", nil, map[string]string{"tls.local": "tls-svc"})

	syncIngresses(t, c)

	require.True(t, haproxy.SSLPassthrough,
		"a service asking for passthrough must turn the topology on, or no ssl frontend serves it")
	require.Equal(t, "tcp", backendMode(t, c, "tls-svc"))
	require.Contains(t, mapFileContent(t, c, "sni"), "tls.local")
}

// TestServiceAnnotationOverridesTheIngressOne is what makes the annotation usable to
// settle a disagreement: a backend has a single mode, so two ingresses sharing a service
// port must agree on it, and the service is the one place where that agreement can be
// declared. The ingress asks for passthrough and does not get it.
func TestServiceAnnotationOverridesTheIngressOne(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	passthroughService(t, c, ns, "clear-svc", map[string]string{"ssl-passthrough": "false"})
	registerRoutedIngress(ns, "ing", map[string]string{"ssl-passthrough": "true"},
		map[string]string{"clear.local": "clear-svc"})

	syncIngresses(t, c)

	require.False(t, haproxy.SSLPassthrough,
		"no path resolving to passthrough must leave the layer 4 topology off")
	require.Equal(t, "http", backendMode(t, c, "clear-svc"))
	require.NotContains(t, mapFileContent(t, c, "sni"), "clear.local",
		"a route the service declined to serve in passthrough must not be indexed in sni.map")
}

// TestOneIngressCanMixBothModes covers what resolving per path buys, and what was not
// expressible while the mode was decided once per ingress: a single ingress fronting a
// service which terminates TLS itself and another which does not. The two hosts must end
// up in different maps, since sni.map is what makes the ssl frontend short-circuit the
// offload path.
func TestOneIngressCanMixBothModes(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	passthroughService(t, c, ns, "tls-svc", map[string]string{"ssl-passthrough": "true"})
	passthroughService(t, c, ns, "clear-svc", nil)
	registerRoutedIngress(ns, "ing", nil, map[string]string{"tls.local": "tls-svc", "clear.local": "clear-svc"})

	syncIngresses(t, c)

	require.Equal(t, "tcp", backendMode(t, c, "tls-svc"))
	require.Equal(t, "http", backendMode(t, c, "clear-svc"),
		"a path pointing at a service which does not ask for passthrough must stay in http mode, "+
			"even when another path of the same ingress does")

	sni := mapFileContent(t, c, "sni")
	require.Contains(t, sni, "tls.local")
	require.NotContains(t, sni, "clear.local")
	require.Contains(t, mapFileContent(t, c, "path-prefix"), "clear.local/	",
		"the cleartext path must keep its layer 7 route")
}

// TestDefaultBackendIsNeverServedInPassthrough answers a question the per-path resolution
// raises: Kubernetes lets an ingress declare a spec.defaultBackend instead of rules, and
// that default backend does point at a service, which may carry the annotation. It must
// still not be served in passthrough, nor turn the layer 4 topology on.
//
// SetDefaultBackend takes the mode from the frontend it attaches the backend to, so the
// backend is in http mode whatever its service asks for - and that is a guard: a static
// default_backend from an http frontend to a tcp backend is refused by haproxy at parse
// time. Nothing better is available either, passthrough routing being keyed on sni.map,
// which holds host entries only, while a default backend is what serves the requests no
// host matched.
func TestDefaultBackendIsNeverServedInPassthrough(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	passthroughService(t, c, ns, "tls-svc", map[string]string{"ssl-passthrough": "true"})
	ing := registerDefaultBackendIngress(c.store, ns, "ing", "tls-svc")
	t.Cleanup(func() { haproxy.SSLPassthrough = false })

	c.processSSLPassthroughInConfigFile()
	require.False(t, haproxy.SSLPassthrough,
		"the service of a default backend must not turn the layer 4 topology on, no sni entry can route to it")

	require.NoError(t, c.haproxy.APIStartTransaction())
	c.store.BackendsProcessed = map[string]store.BackendOwner{}
	c.defaultBackend = nil
	c.considerDefaultBackend(ing)
	c.setIngressDefaultBackend()
	require.NoError(t, c.haproxy.APICommitTransaction())
	c.haproxy.APIDisposeTransaction()

	require.Equal(t, "http", backendMode(t, c, "tls-svc"),
		"a default backend takes the mode of the frontends it is attached to")
	frontend, err := c.haproxy.FrontendGet(c.haproxy.FrontHTTP)
	require.NoError(t, err)
	require.Equal(t, "ns_svc_tls-svc_http", frontend.DefaultBackend,
		"and it must still be applied: the annotation is ignored, not the default backend")
}
