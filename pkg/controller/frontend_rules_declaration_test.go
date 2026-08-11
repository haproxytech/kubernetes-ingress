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
	"time"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// withTLS gives the ingress a spec.tls section, which is enough to enable the ssl-redirect
// rule on its own: httpsRedirect.Process creates it when an ingress carries TLS secrets,
// with no annotation involved.
func withTLS(ing *store.Ingress, host string) *store.Ingress {
	ing.TLS = map[string]*store.IngressTLS{
		host: {Host: host, SecretName: "example-ingress-tls"},
	}
	return ing
}

// frontendRuleConditions returns the condition of every http-request rule of a frontend, as
// generated in the configuration. Rules land in the in-memory frontend once RefreshRules has
// run, which is where they are asserted rather than in the written file.
func frontendRuleConditions(t *testing.T, c *HAProxyController, frontend string) []string {
	t.Helper()
	c.haproxy.RefreshRules(c.haproxy.HAProxyClient)
	ft, err := c.haproxy.FrontendGet(frontend)
	require.NoError(t, err)
	conditions := make([]string, 0, len(ft.HTTPRequestRuleList))
	for _, rule := range ft.HTTPRequestRuleList {
		conditions = append(conditions, rule.CondTest)
	}
	return conditions
}

// TestRefusedIngressDeclaresNoFrontendRule is the regression test for a rule declared for an
// ingress which ends up with no route.
//
// An ingress-scoped rule tests its own id against the map value the request resolves to, and
// that id gets into a map value through the route. An ingress whose backend was constituted
// by another one in the other mode is deliberately not routed, so nothing carries its ids:
// its rules used to stay in the generated configuration, conditioned on an id no map value
// can hold. They could never match, yet they read as configured - which is exactly how the
// defect was found, by reading haproxy.cfg and finding a redirect rule with no counterpart in
// path-prefix.map.
//
// The shape is the one measured on a live cluster: two ingresses on the same host, the same
// path and the same service port, the older asking for ssl-passthrough and the other carrying
// spec.tls.
func TestRefusedIngressDeclaresNoFrontendRule(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "nginx-svc")
	same := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	registerSharedIngress(ns, "ingress-nginx", "example.ingress", "nginx-svc", true, same)
	withTLS(registerSharedIngress(ns, "ingress-nginx-2", "example.ingress", "nginx-svc", false, same),
		"example.ingress")

	require.Equal(t, "tcp", processIngresses(t, c, "ns_svc_nginx-svc_http"),
		"the oldest ingress owns the backend, so it is in passthrough mode")

	require.Empty(t, frontendRuleConditions(t, c, "http"),
		"the ingress which lost the mode is not routed, so its ssl-redirect rule must not be declared")
	// An id is appended to the map value as "<backend>.<id>", so the backend name followed by
	// a dot is what betrays one. Asserting on "." alone would match the host in the key.
	require.NotContains(t, mapContent(t, c, "path-prefix"), "ns_svc_nginx-svc_http.",
		"no rule id must appear in a map value either")
}

// TestRoutedIngressDeclaresItsFrontendRules pins the negative, which is what an over-eager fix
// would break: an ingress which is served keeps its rules, and its route carries their ids.
// Asserting both halves together is the point - a rule whose id is in no map value is inert,
// and a map value carrying an id no rule tests is equally useless.
func TestRoutedIngressDeclaresItsFrontendRules(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "nginx-svc")
	same := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	withTLS(registerSharedIngress(ns, "ingress-nginx", "example.ingress", "nginx-svc", false, same),
		"example.ingress")

	require.Equal(t, "http", processIngresses(t, c, "ns_svc_nginx-svc_http"))

	conditions := frontendRuleConditions(t, c, "http")
	require.Len(t, conditions, 1, "the ssl-redirect rule of a served ingress must be declared")
	require.Contains(t, conditions[0], "-m dom ", "and stay scoped to the ingress traffic")

	// The id the rule tests has to be the one the route carries, or the rule is inert.
	id := conditions[0][len("{ var(txn.path_match) -m dom ") : len(conditions[0])-3]
	require.Contains(t, mapContent(t, c, "path-prefix"), id,
		"the route must carry the id of the rule, or it can never match")
}

// TestOneRoutedPathIsEnoughToDeclareTheRules covers the boundary of the condition: the rules
// belong to the ingress, not to a path, so a single path getting a route is enough for them to
// be declared. The refused path simply does not carry their ids.
func TestOneRoutedPathIsEnoughToDeclareTheRules(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "owned")
	addBackendService(t, c.store, ns, "free")
	same := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// An older ingress owns the 'owned' backend in passthrough mode.
	registerSharedIngress(ns, "ingress-a-owner", "owner.local", "owned", true, same)
	// The newcomer disagrees on that one, but also serves a host of its own.
	newcomer := withTLS(registerSharedIngress(ns, "ingress-b", "conflict.local", "owned", false, same),
		"conflict.local")
	newcomer.Rules["free.local"] = &store.IngressRule{
		Host: "free.local",
		Paths: map[string]*store.IngressPath{
			"/": {
				Path: "/", PathTypeMatch: store.PATH_TYPE_PREFIX,
				SvcNamespace: ns.Name, SvcName: "free", SvcPortString: "http",
			},
		},
	}

	require.Equal(t, "tcp", processIngresses(t, c, "ns_svc_owned_http"))

	require.Len(t, frontendRuleConditions(t, c, "http"), 1,
		"one routed path is enough: the rules are the ingress', not the path's")
	paths := mapContent(t, c, "path-prefix")
	require.Contains(t, paths, "free.local/", "the path which could be routed must be routed")
	require.NotContains(t, paths, "conflict.local", "the path in conflict must not be")
}
