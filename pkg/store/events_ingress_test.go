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

package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func TestEventIngressPreservesPendingAddition(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	ns := k.GetNamespace("ns")
	uid := types.UID(t.Name())
	created := ingressOn("app", "svc", time.Time{})
	require.True(t, k.EventIngress(ns, created, uid, "1"))

	// Finalizers and other metadata are absent from IngressCore. These events
	// therefore compare equal, but the first reconciliation is still pending.
	for _, resourceVersion := range []string{"2", "3"} {
		updated := ingressOn("app", "svc", time.Time{})
		updated.Status = MODIFIED

		require.True(t, k.EventIngress(ns, updated, uid, resourceVersion),
			"an unchanged ingress still needs its pending first reconciliation")
		require.Equal(t, ADDED, updated.Status)
		require.Same(t, updated, ns.Ingresses["app"])
	}
}

func TestEventIngressUnchangedAfterCleanNeedsNoUpdate(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	ns := k.GetNamespace("ns")
	uid := types.UID(t.Name())
	created := ingressOn("app", "svc", time.Time{})
	require.True(t, k.EventIngress(ns, created, uid, "1"))

	k.Clean()
	require.Equal(t, EMPTY, created.Status)

	updated := ingressOn("app", "svc", time.Time{})
	updated.Status = MODIFIED
	require.False(t, k.EventIngress(ns, updated, uid, "2"),
		"a completed reconciliation has consumed the pending creation")
	require.Equal(t, EMPTY, updated.Status)
}

func TestEventIngressDeleteRemovesPendingAddition(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	ns := k.GetNamespace("ns")
	uid := types.UID(t.Name())
	created := ingressOn("app", "svc", time.Time{})
	require.True(t, k.EventIngress(ns, created, uid, "1"))
	require.Len(t, k.IngressesByService["ns/svc"].Items(), 1)

	deleted := ingressOn("app", "svc", time.Time{})
	deleted.Status = DELETED
	require.True(t, k.EventIngress(ns, deleted, uid, "2"))
	require.NotContains(t, ns.Ingresses, "app")
	require.Empty(t, k.IngressesByService["ns/svc"].Items())
}
