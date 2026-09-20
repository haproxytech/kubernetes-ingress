// Copyright 2026 HAProxy Technologies LLC
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

package gateway

import (
	"testing"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// fakeRouteStatus records not-allowed decisions without a Kubernetes client.
type fakeRouteStatus struct {
	StatusManager
	notAllowed []string
}

func (f *fakeRouteStatus) SetRouteReasonNotAllowedByListeners(msg string, _ store.ParentRef) {
	f.notAllowed = append(f.notAllowed, msg)
}

func selectorListener(selector *store.LabelSelector) store.Listener {
	from := string(v1alpha2.NamespacesFromSelector)
	group := v1alpha2.GroupName
	return store.Listener{
		Name:        "tcp",
		GwNamespace: "gw",
		GwName:      "gateway",
		AllowedRoutes: &store.AllowedRoutes{
			Kinds: []store.RouteGroupKind{{Group: &group, Kind: K8S_TCPROUTE_KIND}},
			Namespaces: &store.RouteNamespaces{
				From:     &from,
				Selector: selector,
			},
		},
	}
}

// TestTCPRouteAllowedByListenerNamespaceSelector is the regression test for the
// AllowedRoutes namespace selector: it must follow the live namespace labels, so
// changing the `routes` label moves a TCPRoute in and out of the listener.
func TestTCPRouteAllowedByListenerNamespaceSelector(t *testing.T) {
	t.Parallel()

	const (
		gatewayNamespace = "gw"
		routeNamespace   = "app"
	)
	parentRef := store.ParentRef{Namespace: ptr("gw"), Name: "gateway"}
	st := store.K8s{
		Namespaces: map[string]*store.Namespace{
			routeNamespace: {Name: routeNamespace, Labels: map[string]string{"routes": "allowed"}},
		},
	}
	status := &fakeRouteStatus{}
	gm := GatewayManagerImpl{k8sStore: st, statusManager: status}
	listener := selectorListener(&store.LabelSelector{MatchLabels: map[string]string{"routes": "allowed"}})

	if !gm.isTCPRouteAllowedByListener(listener, routeNamespace, gatewayNamespace, parentRef) {
		t.Fatal("route must be allowed while the namespace carries the selector label")
	}
	if len(status.notAllowed) != 0 {
		t.Fatalf("unexpected not-allowed status: %v", status.notAllowed)
	}

	st.Namespaces[routeNamespace].Labels = map[string]string{"routes": "denied"}
	if gm.isTCPRouteAllowedByListener(listener, routeNamespace, gatewayNamespace, parentRef) {
		t.Fatal("route must be rejected after the namespace label changes")
	}
	if len(status.notAllowed) != 1 {
		t.Fatalf("label change must record exactly one not-allowed status, got %d", len(status.notAllowed))
	}
}

// TestTCPRouteAllowedByListenerNamespaceModes covers the non-selector modes so
// the label-based test cannot pass by accident through a different branch.
func TestTCPRouteAllowedByListenerNamespaceModes(t *testing.T) {
	t.Parallel()

	const (
		gatewayNamespace = "gw"
		routeNamespace   = "app"
	)
	parentRef := store.ParentRef{Namespace: ptr("gw"), Name: "gateway"}
	st := store.K8s{
		Namespaces: map[string]*store.Namespace{
			routeNamespace: {Name: routeNamespace, Labels: map[string]string{}},
		},
	}
	gm := GatewayManagerImpl{k8sStore: st, statusManager: &fakeRouteStatus{}}

	all := string(v1alpha2.NamespacesFromAll)
	listener := selectorListener(nil)
	listener.AllowedRoutes.Namespaces.From = &all
	if !gm.isTCPRouteAllowedByListener(listener, routeNamespace, gatewayNamespace, parentRef) {
		t.Fatal("NamespaceSelector mode All must accept a route from another namespace")
	}

	same := string(v1alpha2.NamespacesFromSame)
	listener.AllowedRoutes.Namespaces.From = &same
	if gm.isTCPRouteAllowedByListener(listener, routeNamespace, gatewayNamespace, parentRef) {
		t.Fatal("Same must reject a route from another namespace")
	}
	if !gm.isTCPRouteAllowedByListener(listener, gatewayNamespace, gatewayNamespace, parentRef) {
		t.Fatal("Same must accept a route from the gateway namespace")
	}
}

func ptr[T any](v T) *T {
	return &v
}
