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

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/status"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

const ourController = "haproxy.org/ingress-controller/haproxy"

// controllerWithClass builds a controller answering to --ingress.class=haproxy, with a status
// manager: manageIngress queues supported ingresses for a status update, and the field is nil
// in the bare harness. Updates are disabled, so the nil client is never reached.
func controllerWithClass(t *testing.T) *HAProxyController {
	t.Helper()
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	c.osArgs.IngressClass = "haproxy"
	c.updateStatusManager = status.New(nil, "haproxy", false, true)
	return c
}

// declareIngressClass registers an IngressClass resource with the given spec.controller.
func declareIngressClass(c *HAProxyController, name, controller string) *store.IngressClass {
	class := &store.IngressClass{Name: name, Controller: controller, Status: store.ADDED}
	c.store.IngressClasses[name] = class
	return class
}

// wrapIngress wraps a store ingress the way the controller does.
func wrapIngress(c *HAProxyController, ing *store.Ingress) *ingress.Ingress {
	return ingress.New(ing, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, c.annotations)
}

// reconcile runs one pass of the walk the controller performs over the store.
func reconcile(t *testing.T, c *HAProxyController) {
	t.Helper()
	require.NoError(t, c.haproxy.APIStartTransaction())
	c.store.BackendsProcessed = map[string]store.BackendOwner{}
	c.store.RoutesProcessedByMapFile = map[string]map[string]store.RouteOwner{}
	c.defaultBackend = nil
	c.processIngressesDefaultImplementation()
	require.NoError(t, c.haproxy.APICommitTransaction())
	c.haproxy.APIDisposeTransaction()
}

// TestIngressServedWhenClassNameDivergesFromFlag is the first scenario of
// ingressclass-matching-issue/README.md: the IngressClass points at this controller, but the
// name of the class resource differs from --ingress.class. Matching is on the class's
// spec.controller, so the ingress is ours and must be served. The informer used to drop it on
// the name comparison, before it could reach the store.
func TestIngressServedWhenClassNameDivergesFromFlag(t *testing.T) {
	c := controllerWithClass(t)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "app")
	declareIngressClass(c, "haproxy-external", ourController)

	ing := registerRoute(ns, "app-ing", "example.ingress", "/", store.PATH_TYPE_PREFIX, "app")
	ing.Class = "haproxy-external"

	reconcile(t, c)

	_, err := c.haproxy.BackendGet("ns_svc_app_http")
	require.NoError(t, err,
		"the IngressClass spec.controller matches this controller, so the ingress is ours whatever the class is named")
}

// TestIngressAdmittedWhenItsClassBecomesOurs is the second scenario, and the one which cannot
// be worked around: an ingress refused on a foreign IngressClass must be picked up when that
// class is handed over to us, without the ingress object changing.
//
// Two assertions, and they do not depend on the same thing. Being served depends on the
// ingress being in the store and Supported saying yes. The Status going back to ADDED depends
// on Ingress.Admit, and that is what gets the LoadBalancer status published: manageIngress
// queues it on Status == ADDED, which nothing else would set here, the class change leaving
// the ingress object untouched. Neutralising the latch fails the second assertion only.
func TestIngressAdmittedWhenItsClassBecomesOurs(t *testing.T) {
	c := controllerWithClass(t)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "app")
	class := declareIngressClass(c, "shared", "example.com/other-controller")

	ing := registerRoute(ns, "app-shared", "example.ingress", "/", store.PATH_TYPE_PREFIX, "app")
	ing.Class = "shared"

	reconcile(t, c)

	_, err := c.haproxy.BackendGet("ns_svc_app_http")
	require.Error(t, err, "the class belongs to another controller, so nothing must be configured for it")
	require.True(t, ing.Ignored, "and the ingress must be recorded as ignored, which is what allows re-admission later")

	// The platform team hands the class over to us. The ingress object itself does not change,
	// so no Ingress event follows: Status is what a sync leaves behind, not ADDED.
	class.Controller = ourController
	ing.Status = store.EMPTY

	reconcile(t, c)

	_, err = c.haproxy.BackendGet("ns_svc_app_http")
	require.NoError(t, err, "the class now points at this controller, so the stored ingress must be served")
	require.False(t, ing.Ignored)
	require.Equal(t, store.ADDED, ing.Status,
		"re-admission must set Status back to ADDED, which is what queues the LoadBalancer status update")
}

// TestSupportedDoesNotRecordAnything pins the split: a caller which only asks the question -
// the status write path, which does so from a goroutine - must leave the store untouched.
func TestSupportedDoesNotRecordAnything(t *testing.T) {
	c := controllerWithClass(t)
	ns := c.store.GetNamespace("ns")
	addBackendService(t, c.store, ns, "app")
	declareIngressClass(c, "shared", "example.com/other-controller")

	ing := registerRoute(ns, "app-shared", "example.ingress", "/", store.PATH_TYPE_PREFIX, "app")
	ing.Class = "shared"
	ing.Ignored = false
	ing.Status = store.EMPTY

	require.False(t, wrapIngress(c, ing).Supported(c.store))
	require.False(t, ing.Ignored, "Supported must not record the decision")
	require.Equal(t, store.EMPTY, ing.Status, "nor touch the status")

	require.False(t, wrapIngress(c, ing).Admit(c.store))
	require.True(t, ing.Ignored, "Admit is the one which records it")
}
