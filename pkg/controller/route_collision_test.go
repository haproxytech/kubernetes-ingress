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
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/fs"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// capturedLogs redirects the logger, which writes through the standard log package, and
// returns what it produced during fn.
func capturedLogs(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	fn()
	return buf.String()
}

// registerRoute adds an ingress routing one host and path to the http port of a service.
func registerRoute(ns *store.Namespace, name, host, path string, pathType string, svcName string) *store.Ingress {
	ing := &store.Ingress{
		IngressCore: store.IngressCore{
			Namespace: ns.Name,
			Name:      name,
			Rules: map[string]*store.IngressRule{
				host: {
					Host: host,
					Paths: map[string]*store.IngressPath{
						path: {
							Path:          path,
							PathTypeMatch: pathType,
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
	ns.Ingresses[name] = ing
	return ing
}

// syncOnce runs one reconciliation over the ingresses of the store, in name order so the walk
// is reproducible, and returns what was logged. It borrows the controller's own sortedByKey:
// name order, not the age order the reconciliation uses, since these tests are about the
// collision report and not about the walk.
func syncOnce(t *testing.T, c *HAProxyController) string {
	t.Helper()
	return capturedLogs(t, func() {
		require.NoError(t, c.haproxy.APIStartTransaction())
		c.store.BackendsProcessed = map[string]struct{}{}
		c.store.RoutesProcessedByMapFile = map[string]map[string]store.RouteOwner{}
		for _, namespace := range c.store.Namespaces {
			for _, ing := range sortedByKey(namespace.Ingresses) {
				ingress.New(ing, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, c.annotations).
					Update(c.store, c.haproxy, c.annotations)
			}
		}
		require.NoError(t, c.haproxy.APICommitTransaction())
		c.haproxy.APIDisposeTransaction()
	})
}

// TestRouteKeyCollisionIsReported is the case found on a live cluster: two ingresses
// declaring the same host and path. Both write a row with the same key, MapAppend having no
// collision check, and haproxy answers with the first matching row of the sorted file - so
// one of the two declarations is silently unreachable.
func TestRouteKeyCollisionIsReported(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "alpha")
	addBackendService(t, c.store, ns, "beta")

	registerRoute(ns, "ing-a", "example.ingress", "/path1", store.PATH_TYPE_PREFIX, "beta")
	registerRoute(ns, "ing-b", "example.ingress", "/path1", store.PATH_TYPE_PREFIX, "alpha")

	logs := syncOnce(t, c)

	require.Contains(t, logs, "routing key 'example.ingress/path1'")
	require.Contains(t, logs, "ns/ing-a", "the report must name the ingress which declared the key first")
	require.Contains(t, logs, "ns/ing-b", "and the one which declared it second")
	require.Contains(t, logs, "only 'ns_svc_alpha_http' is ever used",
		"the value in effect is the lowest one, the rows being sorted, and it must be named")
}

// TestCollisionIsReportedAcrossPathTypes is the case a registry keyed on the path type would
// miss: Prefix and ImplementationSpecific produce the same path-prefix-exact key, so the two
// rows collide although the ingresses declare different path types. Detecting on the rows
// themselves rather than on the declaration is what covers it.
func TestCollisionIsReportedAcrossPathTypes(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "alpha")
	addBackendService(t, c.store, ns, "beta")

	registerRoute(ns, "ing-a", "example.ingress", "/path1", store.PATH_TYPE_PREFIX, "beta")
	registerRoute(ns, "ing-b", "example.ingress", "/path1", store.PATH_TYPE_IMPLEMENTATION_SPECIFIC, "alpha")

	logs := syncOnce(t, c)

	require.Contains(t, logs, "of map 'path-prefix-exact'",
		"the two path types share the path-prefix-exact key, and that collision must be reported")
}

// TestSNIKeyCollisionIsReported covers the layer 4 index, where the key is the host alone: an
// ingress in passthrough mode declaring two paths on one host towards two different service
// ports produces two sni rows with the same key. Only one of them can ever be used, and the
// path is not part of the key, so the second service is unreachable over the passthrough port.
func TestSNIKeyCollisionIsReported(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "alpha")
	addBackendService(t, c.store, ns, "beta")

	ing := registerRoute(ns, "ing-a", "example.ingress", "/path1", store.PATH_TYPE_PREFIX, "beta")
	ing.Annotations = map[string]string{"ssl-passthrough": "true"}
	ing.Rules["example.ingress"].Paths["/path2"] = &store.IngressPath{
		Path: "/path2", PathTypeMatch: store.PATH_TYPE_PREFIX,
		SvcNamespace: ns.Name, SvcName: "alpha", SvcPortString: "http",
	}

	logs := syncOnce(t, c)

	require.Contains(t, logs, "of map 'sni'",
		"two paths of one host in passthrough collide on the sni key, which holds no path")
}

// TestIdenticalRowsAreNotReported pins the boundary: two ingresses declaring the same key
// with the same value are not a problem, whichever row haproxy keeps. host.map is the
// systematic instance, its value being the host itself, so warning on identical rows would
// warn on every ingress sharing a host - which is legitimate and common.
func TestIdenticalRowsAreNotReported(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "alpha")

	registerRoute(ns, "ing-a", "example.ingress", "/path1", store.PATH_TYPE_PREFIX, "alpha")
	registerRoute(ns, "ing-b", "example.ingress", "/path1", store.PATH_TYPE_PREFIX, "alpha")

	require.NotContains(t, syncOnce(t, c), "routing key",
		"the same key resolving to the same value is not a collision worth a warning")
}

// TestDistinctRoutesAreNotReported is the negative that keeps the detection from firing on
// ordinary configurations: two ingresses on the same host but different paths share the
// host.map row and nothing else.
func TestDistinctRoutesAreNotReported(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "alpha")
	addBackendService(t, c.store, ns, "beta")

	registerRoute(ns, "ing-a", "example.ingress", "/path1", store.PATH_TYPE_PREFIX, "alpha")
	registerRoute(ns, "ing-b", "example.ingress", "/path2", store.PATH_TYPE_PREFIX, "beta")

	require.NotContains(t, syncOnce(t, c), "routing key",
		"different paths on one host are not a collision")
}

// TestOneIngressDeclaringSeveralPathsIsNotReported guards against reporting an ingress
// against itself: every path of a host re-declares the host.map row, and an ingress cannot
// collide with its own declaration.
func TestOneIngressDeclaringSeveralPathsIsNotReported(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "alpha")

	ing := registerRoute(ns, "ing-a", "example.ingress", "/path1", store.PATH_TYPE_PREFIX, "alpha")
	ing.Rules["example.ingress"].Paths["/path2"] = &store.IngressPath{
		Path: "/path2", PathTypeMatch: store.PATH_TYPE_PREFIX,
		SvcNamespace: ns.Name, SvcName: "alpha", SvcPortString: "http",
	}

	require.NotContains(t, syncOnce(t, c), "routing key",
		"an ingress declaring several paths of a host must not be reported against itself")
}

// syncInOrder runs one reconciliation over the given ingresses, in the order given, and
// returns what was logged. Unlike syncOnce it does not sort: the point is to control the walk
// order, which the controller does not on this branch - namespaces and ingresses are walked in
// Go map order.
func syncInOrder(t *testing.T, c *HAProxyController, ingresses ...*store.Ingress) string {
	t.Helper()
	return capturedLogs(t, func() {
		require.NoError(t, c.haproxy.APIStartTransaction())
		c.store.BackendsProcessed = map[string]struct{}{}
		c.store.RoutesProcessedByMapFile = map[string]map[string]store.RouteOwner{}
		for _, ing := range ingresses {
			ingress.New(ing, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, c.annotations).
				Update(c.store, c.haproxy, c.annotations)
		}
		require.NoError(t, c.haproxy.APICommitTransaction())
		c.haproxy.APIDisposeTransaction()
	})
}

// TestReportIsTheSameWhateverTheWalkOrder pins what makes the report usable in a log: the
// ingresses are walked in map order, so which of the two colliding declarations is met first
// changes from one reconciliation to the next. A message worded around "the first one declared"
// would therefore describe the same collision differently every time, and read as a new event.
// Naming the winning value first makes it stable - and it is the answer an operator needs.
func TestReportIsTheSameWhateverTheWalkOrder(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "alpha")
	addBackendService(t, c.store, ns, "beta")

	first := registerRoute(ns, "ing-a", "example.ingress", "/path1", store.PATH_TYPE_PREFIX, "beta")
	second := registerRoute(ns, "ing-b", "example.ingress", "/path1", store.PATH_TYPE_PREFIX, "alpha")

	oneWay := collisionMessages(t, syncInOrder(t, c, first, second))
	otherWay := collisionMessages(t, syncInOrder(t, c, second, first))

	require.Equal(t, oneWay, otherWay,
		"the same collision must produce the same messages whichever ingress the walk met first")
	require.Len(t, oneWay, 2, "a Prefix path collides on both of its rows")
}

// collisionMessages returns every collision message of a captured log, each stripped of the
// prefix the logger adds - which carries a timestamp and the transaction id, so comparing raw
// captures would always differ.
func collisionMessages(t *testing.T, logs string) []string {
	t.Helper()
	messages := []string{}
	for _, line := range strings.Split(logs, "\n") {
		if index := strings.Index(line, "routing key"); index >= 0 {
			messages = append(messages, line[index:])
		}
	}
	require.NotEmpty(t, messages, "no collision reported")
	return messages
}

// TestReportedWinnerIsTheFirstRowOfTheFile is the test that keeps the report honest against the
// maps layer. The message names the value in effect, which it derives from maps.Compare - the
// order the rows are written in. If that order ever changes without the report following, the
// message would describe a file which does not exist. So rather than asserting the rule twice,
// this reads the written map file and checks the message names the value of its first row for
// the colliding key, which is the row haproxy answers with.
func TestReportedWinnerIsTheFirstRowOfTheFile(t *testing.T) {
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "alpha")
	addBackendService(t, c.store, ns, "beta")

	registerRoute(ns, "ing-a", "example.ingress", "/path1", store.PATH_TYPE_PREFIX, "beta")
	registerRoute(ns, "ing-b", "example.ingress", "/path1", store.PATH_TYPE_PREFIX, "alpha")

	messages := collisionMessages(t, syncOnce(t, c))

	c.haproxy.RefreshMaps(c.haproxy.HAProxyClient)
	fs.RunDelayedFuncs()
	fs.Writer.WaitUntilWritesDone()
	content, err := os.ReadFile(filepath.Join(c.haproxy.Env.MapsDir, "path-prefix.map"))
	require.NoError(t, err)

	served := ""
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "example.ingress/path1/\t") {
			served = strings.TrimSpace(line[strings.LastIndex(line, "\t")+1:])
			break // haproxy answers with the first matching row
		}
	}
	require.NotEmpty(t, served, "the colliding key must be in the written file")

	require.Contains(t, messages[len(messages)-1], "only '"+served+"' is ever used",
		"the value named as in effect must be the one of the first row of the written file")
}
