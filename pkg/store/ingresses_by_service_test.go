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

package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func ingressOn(name, svcName string, age time.Time) *Ingress {
	return &Ingress{
		IngressCore: IngressCore{
			Namespace:    "ns",
			Name:         name,
			CreationTime: age,
			Rules: map[string]*IngressRule{
				"h": {
					Host: "h",
					Paths: map[string]*IngressPath{
						"/": {Path: "/", SvcNamespace: "ns", SvcName: svcName},
					},
				},
			},
		},
		Status: ADDED,
	}
}

func ingressNames(set *utils.OrderedSet[string, *Ingress]) []string {
	names := make([]string, 0)
	for _, ing := range set.Items() {
		names = append(names, ing.Name)
	}
	return names
}

// TestIngressesByServiceAreOrderedOldestFirst pins the direction of the ordered set, which
// is load bearing twice over: the first ingress processed constitutes a shared backend and
// owns its definition, and the first annotation value found is the one kept when the
// annotations of every ingress of a service are merged. Both must land on the established
// ingress, so the oldest has to come first.
func TestIngressesByServiceAreOrderedOldestFirst(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	ns := k.GetNamespace("ns")

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Registered newest first, and named so that a lexicographic order would disagree.
	k.EventIngress(ns, ingressOn("a-newest", "svc", base.Add(2*time.Hour)), types.UID("1"), "1")
	k.EventIngress(ns, ingressOn("m-middle", "svc", base.Add(time.Hour)), types.UID("2"), "1")
	k.EventIngress(ns, ingressOn("z-oldest", "svc", base), types.UID("3"), "1")

	require.Equal(t, []string{"z-oldest", "m-middle", "a-newest"},
		ingressNames(k.IngressesByService["ns/svc"]),
		"the ordered set must yield the oldest ingress first, whatever the insertion or name order")
}

// TestIngressesByServiceFallBackToNameOnEqualAge covers the common tie: Kubernetes stores
// creationTimestamp with second granularity, so ingresses applied together carry the same
// one and the order would otherwise differ between two controller instances.
func TestIngressesByServiceFallBackToNameOnEqualAge(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	ns := k.GetNamespace("ns")

	same := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	k.EventIngress(ns, ingressOn("b-second", "svc", same), types.UID("1"), "1")
	k.EventIngress(ns, ingressOn("a-first", "svc", same), types.UID("2"), "1")

	require.Equal(t, []string{"a-first", "b-second"},
		ingressNames(k.IngressesByService["ns/svc"]),
		"on equal creation time the name must settle the order")
}
