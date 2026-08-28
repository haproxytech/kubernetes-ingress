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

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// storedIngress puts one ingress in the store and returns it.
func storedIngress(k *K8s, name string) *Ingress {
	ns := k.GetNamespace("ns")
	ing := &Ingress{IngressCore: IngressCore{Namespace: "ns", Name: name}, Status: ADDED}
	ns.Ingresses[name] = ing
	return ing
}

// TestCleanConsumesTheClassChange pins what ClassUpdated means: the class of this ingress
// changed in the event just processed. Like Status, it describes the event and not the
// resource, so a sync consumes it.
//
// Nothing else resets it. An incoming event only clears it by replacing the stored object,
// which needs an event to happen at all - and the case the flag exists for, an IngressClass
// handed over to us, produces no Ingress event. So an ingress whose class was edited once
// stayed flagged until a restart, and manageIngress queued a status update for it on every
// single reconciliation.
func TestCleanConsumesTheClassChange(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	ing := storedIngress(&k, "app")
	ing.ClassUpdated = true

	k.Clean()

	require.Equal(t, EMPTY, ing.Status)
	require.False(t, ing.ClassUpdated,
		"ClassUpdated describes the event, not the resource: a sync must consume it as it consumes Status")
}

// TestCleanConsumesTheRequestToPublishEveryIngress covers the flag of the same nature raised
// by a publish service event. The status manager cannot reset it: handlers receive the store by
// value, so its assignment was a no-op and every sync took the sweep branch from the first
// publish service event on, walking every ingress of every watched namespace forever.
//
// The output was unaffected - the resolution drops everything already correct, so no API call -
// which is why it went unnoticed. The cost was not.
func TestCleanConsumesTheRequestToPublishEveryIngress(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.UpdateAllIngresses = true

	k.Clean()

	require.False(t, k.UpdateAllIngresses,
		"the sync which swept has consumed the request, it must not sweep again on the next one")
}

// TestEventIngressFlagsTheClassChange is the other end of the same flag, and the reason the
// reset cannot be left to the event path: the flag is set on the object which *replaces* the
// stored one, by comparing the two. An ingress nothing touches again is never compared again.
func TestEventIngressFlagsTheClassChange(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	ns := k.GetNamespace("ns")
	storedIngress(&k, "app")

	edited := &Ingress{IngressCore: IngressCore{Namespace: "ns", Name: "app", Class: "haproxy-external"}, Status: MODIFIED}
	require.True(t, k.EventIngress(ns, edited, "uid", "2"))
	require.True(t, edited.ClassUpdated, "the class field differs from the stored one")

	k.Clean()
	require.False(t, ns.Ingresses["app"].ClassUpdated)

	// Same class this time: the flag must not be raised, or every event on the ingress would
	// queue a status update.
	again := &Ingress{IngressCore: IngressCore{Namespace: "ns", Name: "app", Class: "haproxy-external"}, Status: MODIFIED}
	again.Annotations = map[string]string{"whatever": "1"}
	k.EventIngress(ns, again, "uid", "3")
	require.False(t, again.ClassUpdated, "the class did not change, only an annotation did")
}
