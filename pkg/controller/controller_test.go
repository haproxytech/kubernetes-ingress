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

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// newIngress builds a minimal store.Ingress carrying the given annotations.
func newIngress(namespace, name string, faked bool, annotations map[string]string) *store.Ingress {
	return &store.Ingress{
		IngressCore: store.IngressCore{
			Namespace:   namespace,
			Name:        name,
			Annotations: annotations,
		},
		Faked: faked,
	}
}

// newNamespace builds a store.Namespace holding the given ingresses.
func newNamespace(name string, relevant bool, ingresses ...*store.Ingress) *store.Namespace {
	ns := &store.Namespace{
		Name:      name,
		Relevant:  relevant,
		Ingresses: map[string]*store.Ingress{},
	}
	for _, ing := range ingresses {
		ns.Ingresses[ing.Name] = ing
	}
	return ns
}

// newControllerWithStore wires a HAProxyController around a store made of the
// provided namespaces and ConfigMap (global) annotations.
func newControllerWithStore(cfgMapAnnotations map[string]string, namespaces ...*store.Namespace) *HAProxyController {
	nsMap := map[string]*store.Namespace{}
	for _, ns := range namespaces {
		nsMap[ns.Name] = ns
	}
	return &HAProxyController{
		store: store.K8s{
			Namespaces: nsMap,
			ConfigMaps: store.ConfigMaps{
				Main: &store.ConfigMap{Annotations: cfgMapAnnotations},
			},
		},
	}
}

// TestProcessSSLPassthroughInConfigFile guards the fix for the deny/allow-list
// rule flapping bug: the global ssl-passthrough mode must be resolved from the
// whole set of ingresses *before* any rule is built, so the frontend a REQ_DENY
// rule lands on no longer depends on the (randomized) order in which ingresses
// are processed.
//
// The function under test must therefore set haproxy.SSLPassthrough to true if
// and only if at least one *processed* ingress (i.e. one in a relevant namespace
// or a faked one) — or the global ConfigMap — enables ssl-passthrough.
func TestProcessSSLPassthroughInConfigFile(t *testing.T) {
	const annPassthrough = "ssl-passthrough"

	tests := []struct {
		name       string
		cfgMap     map[string]string
		namespaces []*store.Namespace
		want       bool
	}{
		{
			name: "no ingress enables ssl-passthrough",
			namespaces: []*store.Namespace{
				newNamespace(
					"app", true,
					newIngress("app", "ing-a", false, map[string]string{"deny-list": "10.0.0.0/8"}),
					newIngress("app", "ing-b", false, nil),
				),
			},
			want: false,
		},
		{
			name: "one ingress enables ssl-passthrough in a relevant namespace",
			namespaces: []*store.Namespace{
				newNamespace(
					"app", true,
					newIngress("app", "ing-deny", false, map[string]string{"deny-list": "10.0.0.0/8"}),
					newIngress("app", "ing-pt", false, map[string]string{annPassthrough: "true"}),
				),
			},
			want: true,
		},
		{
			name:   "ssl-passthrough enabled globally through the ConfigMap",
			cfgMap: map[string]string{annPassthrough: "true"},
			namespaces: []*store.Namespace{
				newNamespace(
					"app", true,
					newIngress("app", "ing-deny", false, map[string]string{"deny-list": "10.0.0.0/8"}),
				),
			},
			want: true,
		},
		{
			name: "ssl-passthrough explicitly disabled is not enabled",
			namespaces: []*store.Namespace{
				newNamespace(
					"app", true,
					newIngress("app", "ing-pt", false, map[string]string{annPassthrough: "false"}),
				),
			},
			want: false,
		},
		{
			name: "ssl-passthrough on a non-relevant, non-faked ingress is filtered out",
			namespaces: []*store.Namespace{
				newNamespace(
					"excluded", false,
					newIngress("excluded", "ing-pt", false, map[string]string{annPassthrough: "true"}),
				),
			},
			want: false,
		},
		{
			name: "ssl-passthrough on a faked ingress in a non-relevant namespace is honored",
			namespaces: []*store.Namespace{
				newNamespace(
					"excluded", false,
					newIngress("excluded", "ing-pt", true, map[string]string{annPassthrough: "true"}),
				),
			},
			want: true,
		},
		{
			name: "invalid ssl-passthrough value does not enable passthrough",
			namespaces: []*store.Namespace{
				newNamespace(
					"app", true,
					newIngress("app", "ing-pt", false, map[string]string{annPassthrough: "notabool"}),
				),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// SSLPassthrough is a package-level flag; reset it before and after
			// each case to keep the test order-independent and side-effect free.
			haproxy.SSLPassthrough = false
			t.Cleanup(func() { haproxy.SSLPassthrough = false })

			c := newControllerWithStore(tt.cfgMap, tt.namespaces...)
			c.processSSLPassthroughInConfigFile()

			if haproxy.SSLPassthrough != tt.want {
				t.Fatalf("haproxy.SSLPassthrough = %v, want %v", haproxy.SSLPassthrough, tt.want)
			}
		})
	}
}

// TestProcessSSLPassthroughInConfigFileIsOrderIndependent is the direct
// regression test for the flapping bug. The original implementation set the
// global flag while iterating ingresses, so the value observed when a deny-list
// ingress was processed depended on whether the ssl-passthrough ingress had
// already been visited — and Go randomizes map iteration order on every call.
//
// With the dedicated pre-pass, the resolved mode must be stable across many
// runs regardless of that randomization.
func TestProcessSSLPassthroughInConfigFileIsOrderIndependent(t *testing.T) {
	t.Cleanup(func() { haproxy.SSLPassthrough = false })

	// A passthrough ingress buried among many non-passthrough ones, spread over
	// several namespaces so map iteration order varies meaningfully.
	namespaces := []*store.Namespace{}
	for _, nsName := range []string{"ns-a", "ns-b", "ns-c", "ns-d"} {
		ings := []*store.Ingress{}
		for _, ingName := range []string{"deny-1", "deny-2", "deny-3", "deny-4"} {
			ings = append(ings, newIngress(nsName, ingName, false, map[string]string{"deny-list": "10.0.0.0/8"}))
		}
		namespaces = append(namespaces, newNamespace(nsName, true, ings...))
	}
	// Exactly one ingress enables ssl-passthrough.
	namespaces[2].Ingresses["pt"] = newIngress("ns-c", "pt", false, map[string]string{"ssl-passthrough": "true"})

	c := newControllerWithStore(nil, namespaces...)

	const runs = 200
	for i := 0; i < runs; i++ {
		haproxy.SSLPassthrough = false
		c.processSSLPassthroughInConfigFile()
		if !haproxy.SSLPassthrough {
			t.Fatalf("run %d: haproxy.SSLPassthrough = false, want true (resolution must not depend on iteration order)", i)
		}
	}
}
