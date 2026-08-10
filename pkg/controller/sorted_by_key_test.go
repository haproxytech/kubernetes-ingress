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

// TestSortedByKeyIsStable is what makes the ingress walk reproducible: Go randomizes
// map iteration on every pass, and several decisions taken while walking depend on the
// order — the mode of a backend shared by two ingresses above all, since the last one
// processed imposes its whole definition.
func TestSortedByKeyIsStable(t *testing.T) {
	ingresses := map[string]*store.Ingress{
		"zeta":  {IngressCore: store.IngressCore{Name: "zeta"}},
		"alpha": {IngressCore: store.IngressCore{Name: "alpha"}},
		"mu":    {IngressCore: store.IngressCore{Name: "mu"}},
		"beta":  {IngressCore: store.IngressCore{Name: "beta"}},
	}
	want := []string{"alpha", "beta", "mu", "zeta"}

	// Repeated because a single pass over a 4-entry map can accidentally come out
	// sorted; the point is that every pass gives the same order.
	for range 20 {
		got := make([]string, 0, len(ingresses))
		for _, ing := range sortedByKey(ingresses) {
			got = append(got, ing.Name)
		}
		require.Equal(t, want, got, "sortedByKey must order by key on every pass")
	}
}

func TestSortedByKeyEdgeCases(t *testing.T) {
	require.Empty(t, sortedByKey(map[string]*store.Namespace{}))
	// A nil map yields an empty slice rather than nil: callers only range over it.
	require.Empty(t, sortedByKey[*store.Namespace](nil))

	single := map[string]*store.Namespace{"only": {Name: "only"}}
	require.Len(t, sortedByKey(single), 1)
}
