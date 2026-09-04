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
	"time"

	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
)

type mockSessions struct {
	starts []string
	stops  []string
	accept map[uint64]bool
	allow  bool
}

func (m *mockSessions) Start(namespace string) error {
	m.starts = append(m.starts, namespace)
	return nil
}
func (m *mockSessions) Stop(namespace string) { m.stops = append(m.stops, namespace) }
func (m *mockSessions) Accept(_ string, epoch uint64) bool {
	if m.accept != nil {
		if v, ok := m.accept[epoch]; ok {
			return v
		}
	}
	return m.allow
}
func (m *mockSessions) MarkReady(string, uint64) bool { return true }
func (m *mockSessions) Close()                        {}

func waitProcessed(t *testing.T, ch chan k8ssync.SyncDataEvent, ev k8ssync.SyncDataEvent) {
	t.Helper()
	done := make(chan struct{})
	ev.EventProcessed = done
	ch <- ev
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("event was not processed")
	}
}

func TestSyncDataRejectsStaleEpoch(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	st.EventNamespace(nil, &store.Namespace{Name: "app", Status: store.ADDED})
	st.MarkNamespaceReady("app")

	ch := make(chan k8ssync.SyncDataEvent, 4)
	mock := &mockSessions{accept: map[uint64]bool{1: false, 2: true}}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	ing := &store.Ingress{
		IngressCore: store.IngressCore{
			Namespace: "app",
			Name:      "web",
			Rules: map[string]*store.IngressRule{
				"h": {Host: "h", Paths: map[string]*store.IngressPath{"/": {Path: "/", SvcNamespace: "app", SvcName: "svc"}}},
			},
		},
		Status: store.ADDED,
	}
	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.INGRESS, Namespace: "app", Data: ing, NamespaceEpoch: 1, UID: types.UID("1"), ResourceVersion: "1",
	})
	if st.Namespaces["app"].Ingresses["web"] != nil {
		t.Fatal("stale epoch ingress must not be stored")
	}

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.INGRESS, Namespace: "app", Data: ing, NamespaceEpoch: 2, UID: types.UID("1"), ResourceVersion: "1",
	})
	if st.Namespaces["app"].Ingresses["web"] == nil {
		t.Fatal("current epoch ingress must be stored")
	}
}

func TestSyncDataDropsZeroEpochCRTCP(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	st.EventNamespace(nil, &store.Namespace{Name: "app", Status: store.ADDED})
	st.MarkNamespaceReady("app")

	ch := make(chan k8ssync.SyncDataEvent, 4)
	mock := &mockSessions{accept: map[uint64]bool{0: false, 4: true}}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	tcp := &store.TCPs{Name: "tcp", Namespace: "app", Status: store.ADDED}
	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.CR_TCP, Namespace: "app", Name: "tcp", Data: tcp, NamespaceEpoch: 0,
	})
	if st.Namespaces["app"].CRs.TCPsPerCR["tcp"] != nil {
		t.Fatal("late CR_TCP with epoch 0 must be dropped by Accept")
	}

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.CR_TCP, Namespace: "app", Name: "tcp", Data: tcp, NamespaceEpoch: 4,
	})
	if st.Namespaces["app"].CRs.TCPsPerCR["tcp"] == nil {
		t.Fatal("CR_TCP with the session epoch must be stored")
	}
}

func TestSyncDataStartsAndStopsSessions(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	ch := make(chan k8ssync.SyncDataEvent, 4)
	mock := &mockSessions{allow: true}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.ADDED},
	})
	if len(mock.starts) != 1 || mock.starts[0] != "app" {
		t.Fatalf("runtime ADD should Start session, got %v", mock.starts)
	}

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.DELETED},
	})
	if len(mock.stops) != 1 {
		t.Fatalf("DELETE should Stop session, got %v", mock.stops)
	}
	if _, ok := st.Namespaces["app"]; ok {
		t.Fatal("selector DELETE should drop store namespace")
	}
}

func TestSyncDataInitialListDoesNotStartSession(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	ch := make(chan k8ssync.SyncDataEvent, 4)
	mock := &mockSessions{allow: true}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app", IsInInitialList: true,
		Data: &store.Namespace{Name: "app", Status: store.ADDED},
	})
	if len(mock.starts) != 0 {
		t.Fatalf("initial-list ADD must not Start, got %v", mock.starts)
	}
	if _, ok := st.NamespacesAccess.Selected["app"]; !ok {
		t.Fatal("initial-list ADD must still select the namespace")
	}
}

func TestIngressesEligibleForMergeSkipStartingNamespace(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	st.EventNamespace(nil, &store.Namespace{Name: "ready", Status: store.ADDED})
	st.EventNamespace(nil, &store.Namespace{Name: "starting", Status: store.ADDED})
	st.MarkNamespaceReady("ready")

	readyIng := &store.Ingress{
		IngressCore: store.IngressCore{
			Namespace: "ready", Name: "ok",
			Rules: map[string]*store.IngressRule{
				"h": {Host: "ok", Paths: map[string]*store.IngressPath{"/": {Path: "/", SvcNamespace: "ready", SvcName: "svc"}}},
			},
		},
		Status: store.ADDED,
	}
	foreign := &store.Ingress{
		IngressCore: store.IngressCore{
			Namespace: "starting", Name: "bad",
			Rules: map[string]*store.IngressRule{
				"h": {Host: "bad", Paths: map[string]*store.IngressPath{"/": {Path: "/", SvcNamespace: "ready", SvcName: "svc"}}},
			},
		},
		Status: store.ADDED,
	}
	st.EventIngress(st.Namespaces["ready"], readyIng, types.UID("1"), "1")
	st.EventIngress(st.Namespaces["starting"], foreign, types.UID("2"), "1")

	c := &HAProxyController{store: st}
	got := c.ingressesEligibleForMerge(&store.Service{Namespace: "ready", Name: "svc"})
	if len(got) != 1 || got[0].Name != "ok" {
		t.Fatalf("merge must keep only ready-ns ingresses, got %#v", got)
	}
}
