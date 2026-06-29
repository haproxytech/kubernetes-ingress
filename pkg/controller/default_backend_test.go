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

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func defaultBackendIngress(namespace, name string) *store.Ingress {
	return &store.Ingress{
		IngressCore: store.IngressCore{
			Namespace:      namespace,
			Name:           name,
			DefaultBackend: &store.IngressPath{IsDefaultBackend: true},
		},
	}
}

func TestPickDefaultBackendNilHandling(t *testing.T) {
	a := defaultBackendIngress("ns", "a")

	require.Nil(t, pickDefaultBackend(nil, nil))
	require.Same(t, a, pickDefaultBackend(nil, a), "candidate must win when there is no current winner")
	require.Same(t, a, pickDefaultBackend(a, nil), "current winner must be kept when candidate is nil")
}

func TestPickDefaultBackendSmallestKeyWins(t *testing.T) {
	a := defaultBackendIngress("ns", "a")
	b := defaultBackendIngress("ns", "b")

	require.Same(t, a, pickDefaultBackend(b, a), "smaller name must win")
	require.Same(t, a, pickDefaultBackend(a, b), "smaller name must win regardless of argument position")

	nsA := defaultBackendIngress("ns-a", "z")
	nsB := defaultBackendIngress("ns-b", "a")
	require.Same(t, nsA, pickDefaultBackend(nsB, nsA), "namespace is compared before name")
}

// TestPickDefaultBackendIsOrderIndependent is the core property: whatever order the
// candidates are submitted in (Go map iteration is random), the selected default
// backend is always the same one. This is what removes the flaky/non-deterministic
// behavior of the previous per-ingress application.
func TestPickDefaultBackendIsOrderIndependent(t *testing.T) {
	a := defaultBackendIngress("ns", "a")
	b := defaultBackendIngress("ns", "b")
	c := defaultBackendIngress("ns", "c")

	orders := [][]*store.Ingress{
		{a, b, c},
		{c, b, a},
		{b, c, a},
		{c, a, b},
	}

	for _, order := range orders {
		var winner *store.Ingress
		for _, ing := range order {
			winner = pickDefaultBackend(winner, ing)
		}
		require.Same(t, a, winner, "winner must always be the smallest key whatever the submission order")
	}
}
