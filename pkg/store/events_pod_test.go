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

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func TestEventPodTracksPodIPChanges(t *testing.T) {
	store := NewK8sStore(utils.OSArgs{})

	if !store.EventPod(PodEvent{Status: ADDED, Name: "haproxy-ingress-abcde-fghij", Namespace: "default", IP: "10.0.0.1"}) {
		t.Fatal("expected add event to require an update")
	}
	if got := store.HaProxyPods["haproxy-ingress-abcde-fghij"]; got != "10.0.0.1" {
		t.Fatalf("expected pod IP 10.0.0.1, got %q", got)
	}
	if store.EventPod(PodEvent{Status: MODIFIED, Name: "haproxy-ingress-abcde-fghij", Namespace: "default", IP: "10.0.0.1"}) {
		t.Fatal("expected unchanged pod IP to skip update")
	}
	if !store.EventPod(PodEvent{Status: MODIFIED, Name: "haproxy-ingress-abcde-fghij", Namespace: "default", IP: "10.0.0.2"}) {
		t.Fatal("expected changed pod IP to require an update")
	}
	if got := store.HaProxyPods["haproxy-ingress-abcde-fghij"]; got != "10.0.0.2" {
		t.Fatalf("expected pod IP 10.0.0.2, got %q", got)
	}
	if !store.EventPod(PodEvent{Status: DELETED, Name: "haproxy-ingress-abcde-fghij", Namespace: "default"}) {
		t.Fatal("expected delete event to require an update")
	}
	if _, ok := store.HaProxyPods["haproxy-ingress-abcde-fghij"]; ok {
		t.Fatal("expected pod to be removed")
	}
}

func TestEventPodIgnoresEmptyPodIP(t *testing.T) {
	store := NewK8sStore(utils.OSArgs{})

	if store.EventPod(PodEvent{Status: ADDED, Name: "haproxy-ingress-abcde-fghij", Namespace: "default"}) {
		t.Fatal("expected empty pod IP to skip update")
	}
	if len(store.HaProxyPods) != 0 {
		t.Fatalf("expected no tracked pods, got %d", len(store.HaProxyPods))
	}
}
