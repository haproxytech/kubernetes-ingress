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

package controller

import (
	"fmt"
	"testing"

	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// Model the session protocol here; the real manager's epoch and Drain behavior
// are covered in pkg/k8s. This lets tests place RETIRED on either side of ADD.
type replacementSessions struct {
	mockSessions
	epoch uint64
	live  bool
}

func (m *replacementSessions) Start(namespace string) error {
	if m.live {
		return nil
	}
	m.live = true
	m.epoch++
	m.ready = false
	return m.mockSessions.Start(namespace)
}

func (m *replacementSessions) Accept(_ string, epoch uint64) bool {
	return m.live && !m.draining && epoch == m.epoch
}

func (m *replacementSessions) MarkReady(namespace string, epoch uint64) bool {
	if !m.Accept(namespace, epoch) {
		return false
	}
	m.ready = true
	return true
}

func (m *replacementSessions) Drain(namespace string) bool {
	if !m.live {
		return false
	}
	m.ready = false
	return m.mockSessions.Drain(namespace)
}

func (m *replacementSessions) FinishDrain(namespace string) {
	m.mockSessions.FinishDrain(namespace)
	m.live = false
}

func TestSyncDataNamespaceUIDReplacement(t *testing.T) {
	for _, matched := range []bool{true, false} {
		for _, retiredFirst := range []bool{true, false} {
			t.Run(fmt.Sprintf("matched=%t/retiredFirst=%t", matched, retiredFirst), func(t *testing.T) {
				st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
				ch := make(chan k8ssync.SyncDataEvent, 4)
				sessions := &replacementSessions{}
				c := &HAProxyController{store: st, eventChan: ch, sessions: sessions}
				done := make(chan struct{})
				go func() { c.SyncData(); close(done) }()
				defer func() { close(ch); <-done }()
				namespace := func(status store.Status, match bool) {
					waitProcessed(t, ch, k8ssync.SyncDataEvent{
						SyncType: k8ssync.NAMESPACE, Namespace: "app",
						Data: &store.Namespace{Name: "app", Status: status, Labels: map[string]string{"watch": fmt.Sprint(match)}},
					})
				}
				service := func(name string, epoch uint64) {
					waitProcessed(t, ch, k8ssync.SyncDataEvent{
						SyncType: k8ssync.SERVICE, Namespace: "app", NamespaceEpoch: epoch,
						Data: &store.Service{Namespace: "app", Name: name, Status: store.ADDED},
					})
				}
				namespace(store.ADDED, true)
				service("old", 1)
				waitProcessed(t, ch, k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE_SESSION_READY, Namespace: "app", NamespaceEpoch: 1})
				old := st.Namespaces["app"]
				if old == nil || !old.Relevant || old.Services["old"] == nil {
					t.Fatal("initial generation must be ready with its service")
				}

				// This is the DELETE/ADD pair emitted by the UID-aware informer.
				namespace(store.DELETED, true)
				if old.Relevant || !sessions.draining {
					t.Fatal("DELETE must make the old generation dormant and drain its watch")
				}
				service("late-during-drain", 1)
				if old.Services["late-during-drain"] != nil {
					t.Fatal("old epoch mutated the store during Drain")
				}
				retired := func() {
					waitProcessed(t, ch, k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE_WATCH_RETIRED, Namespace: "app", NamespaceEpoch: 1})
				}
				if retiredFirst {
					retired()
					if _, ok := st.Namespaces["app"]; ok {
						t.Fatal("RETIRED must remove the old namespace before replacement ADD")
					}
				}
				namespace(store.ADDED, matched)
				if !retiredFirst {
					if len(sessions.starts) != 1 || st.Namespaces["app"] != old {
						t.Fatal("replacement must wait for RETIRED before restarting")
					}
					_, pending := st.NamespacesAccess.PendingLabels["app"]
					if pending != matched {
						t.Fatalf("pending replacement = %t, want %t", pending, matched)
					}
					retired()
				}
				service("late-after-retired", 1)
				waitProcessed(t, ch, k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE_SESSION_READY, Namespace: "app", NamespaceEpoch: 1})
				if _, pending := st.NamespacesAccess.PendingLabels["app"]; pending {
					t.Fatal("RETIRED must consume pending replacement labels")
				}
				if !matched {
					_, selected := st.NamespacesAccess.Selected["app"]
					if selected || st.Namespaces["app"] != nil || sessions.live || len(sessions.starts) != 1 {
						t.Fatal("unmatched replacement inherited the old sticky selection, watch or resources")
					}
					return
				}
				fresh := st.Namespaces["app"]
				if fresh == nil || fresh == old || fresh.Relevant || sessions.epoch != 2 || len(sessions.starts) != 2 {
					t.Fatal("matching replacement needs a fresh, not-yet-ready generation")
				}
				if len(fresh.Services) != 0 {
					t.Fatal("old resources or stale-epoch events leaked into the replacement")
				}
				service("fresh", 2)
				if fresh.Services["fresh"] == nil || fresh.Relevant {
					t.Fatal("current epoch must warm the store without becoming Relevant before READY")
				}
				waitProcessed(t, ch, k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE_SESSION_READY, Namespace: "app", NamespaceEpoch: 2})
				if !fresh.Relevant {
					t.Fatal("current READY must activate the replacement")
				}
			})
		}
	}
}

func TestSyncDataNamespaceSameUIDKeepsStickySession(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	ch := make(chan k8ssync.SyncDataEvent, 4)
	sessions := &replacementSessions{}
	c := &HAProxyController{store: st, eventChan: ch, sessions: sessions}
	done := make(chan struct{})
	go func() { c.SyncData(); close(done) }()
	defer func() { close(ch); <-done }()
	for i, match := range []bool{true, false, true} {
		status := store.MODIFIED
		if i == 0 {
			status = store.ADDED
		}
		waitProcessed(t, ch, k8ssync.SyncDataEvent{
			SyncType: k8ssync.NAMESPACE, Namespace: "app",
			Data: &store.Namespace{Name: "app", Status: status, Labels: map[string]string{"watch": fmt.Sprint(match)}},
		})
		if i == 0 {
			waitProcessed(t, ch, k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE_SESSION_READY, Namespace: "app", NamespaceEpoch: 1})
		}
		waitProcessed(t, ch, k8ssync.SyncDataEvent{
			SyncType: k8ssync.SERVICE, Namespace: "app", NamespaceEpoch: 1,
			Data: &store.Service{Namespace: "app", Name: fmt.Sprint(i), Status: store.ADDED},
		})
		ns := st.Namespaces["app"]
		_, selected := st.NamespacesAccess.Selected["app"]
		if ns == nil || ns.Relevant != match || !selected || len(ns.Services) != i+1 {
			t.Fatal("same-UID relabel must preserve selection and current-epoch resource updates while toggling relevance")
		}
		if sessions.epoch != 1 || len(sessions.starts) != 1 || len(sessions.stops) != 0 {
			t.Fatal("same-UID relabel must retain the original watch session")
		}
	}
}
