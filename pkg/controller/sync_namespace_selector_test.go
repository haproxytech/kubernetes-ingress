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
	"errors"
	"testing"
	"time"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
)

type mockSessions struct {
	starts   []string
	stops    []string
	accept   map[uint64]bool
	allow    bool
	ready    bool
	calls    []string
	startErr error
	draining bool
}

func (m *mockSessions) Start(namespace string) error {
	m.starts = append(m.starts, namespace)
	m.calls = append(m.calls, "start:"+namespace)
	return m.startErr
}

func (m *mockSessions) Stop(namespace string) {
	m.stops = append(m.stops, namespace)
	m.calls = append(m.calls, "stop:"+namespace)
}

func (m *mockSessions) Accept(_ string, epoch uint64) bool {
	if m.accept != nil {
		if v, ok := m.accept[epoch]; ok {
			return v
		}
	}
	return m.allow
}
func (m *mockSessions) MarkReady(string, uint64) bool { return m.startErr == nil }
func (m *mockSessions) Drain(namespace string) bool {
	m.stops = append(m.stops, namespace)
	m.calls = append(m.calls, "drain:"+namespace)
	if m.startErr != nil {
		return false
	}
	m.draining = true
	return true
}
func (m *mockSessions) Draining(string) bool { return m.draining }
func (m *mockSessions) FinishDrain(string)   { m.draining = false }
func (m *mockSessions) Ready(string) bool    { return m.ready }
func (m *mockSessions) Close()               {}

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
	st.EventNamespace(nil, &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}})
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
	st.EventNamespace(nil, &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}})
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

func TestSyncDataLabelAddSelectsOnModified(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	ch := make(chan k8ssync.SyncDataEvent, 4)
	mock := &mockSessions{allow: true}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{}},
	})
	if len(mock.starts) != 0 {
		t.Fatalf("unmatched ADD must not Start, got %v", mock.starts)
	}
	if _, ok := st.Namespaces["app"]; ok {
		t.Fatal("unmatched ADD must not create a store namespace")
	}

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.MODIFIED, Labels: map[string]string{"watch": "true"}},
	})
	if len(mock.starts) != 1 || mock.starts[0] != "app" {
		t.Fatalf("label MODIFIED must Start, got %v", mock.starts)
	}
	if _, ok := st.NamespacesAccess.Selected["app"]; !ok {
		t.Fatal("label MODIFIED must select the namespace")
	}
	if st.Namespaces["app"] == nil {
		t.Fatal("label MODIFIED must persist the store namespace")
	}
}

func TestSyncDataStartsAndStopsSessions(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	ch := make(chan k8ssync.SyncDataEvent, 8)
	mock := &mockSessions{allow: true}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}},
	})
	if len(mock.starts) != 1 || mock.starts[0] != "app" {
		t.Fatalf("runtime ADD should Start session, got %v", mock.starts)
	}

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.MODIFIED, Labels: map[string]string{}},
	})
	if len(mock.stops) != 0 {
		t.Fatalf("unlabel must not Drain watchers, got %v", mock.stops)
	}
	if st.Namespaces["app"].Relevant {
		t.Fatal("unlabel must set Dormant")
	}

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.DELETED},
	})
	if len(mock.stops) != 1 {
		t.Fatalf("DELETE should Drain session, got %v", mock.stops)
	}
	if _, ok := st.Namespaces["app"]; !ok {
		t.Fatal("DELETE should keep store until RETIRED")
	}

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE_WATCH_RETIRED, Namespace: "app",
	})
	if _, ok := st.Namespaces["app"]; ok {
		t.Fatal("RETIRED should drop store namespace")
	}
}

func TestSyncDataInitialListStartsSession(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	ch := make(chan k8ssync.SyncDataEvent, 4)
	mock := &mockSessions{allow: true}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}},
	})
	if len(mock.starts) != 1 || mock.starts[0] != "app" {
		t.Fatalf("initial-list ADD must Start in the event queue, got %v", mock.starts)
	}
	if _, ok := st.NamespacesAccess.Selected["app"]; !ok {
		t.Fatal("initial-list ADD must still select the namespace")
	}
}

func TestSyncDataBootstrapDeleteAndRelabelAreOrdered(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	ch := make(chan k8ssync.SyncDataEvent, 6)
	mock := &mockSessions{allow: true}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}

	ch <- k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}},
	}
	ch <- k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.DELETED},
	}
	bootstrapDone := make(chan struct{})
	ch <- k8ssync.SyncDataEvent{SyncType: k8ssync.BARRIER, EventProcessed: bootstrapDone}
	close(ch)
	// No HAProxy client is installed: a bootstrap barrier must not generate
	// configuration, even though the namespace events marked the store dirty.
	c.SyncData()
	select {
	case <-bootstrapDone:
	default:
		t.Fatal("bootstrap barrier was not acknowledged")
	}
	if len(mock.calls) != 2 || mock.calls[0] != "start:app" || mock.calls[1] != "drain:app" {
		t.Fatalf("initial ADD and DELETE must execute in order, got %v", mock.calls)
	}
	if _, ok := st.NamespacesAccess.Selected["app"]; !ok {
		t.Fatal("deleted namespace stays selected until RETIRED")
	}

	ch = make(chan k8ssync.SyncDataEvent, 4)
	c.eventChan = ch
	ch <- k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE_WATCH_RETIRED, Namespace: "app"}
	ch <- k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}},
	}
	ch <- k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE_SESSION_READY, Namespace: "app", NamespaceEpoch: 2}
	close(ch)
	c.SyncData()
	if len(mock.calls) < 3 || mock.calls[len(mock.calls)-1] != "start:app" && mock.calls[2] != "start:app" {
		t.Fatalf("recreate after RETIRED must Start, got %v", mock.calls)
	}
}

func TestSyncDataStartErrorDoesNotPanic(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	ch := make(chan k8ssync.SyncDataEvent, 4)
	mock := &mockSessions{allow: true, startErr: errors.New("start failed")}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.MODIFIED, Labels: map[string]string{"watch": "true"}},
	})
	if len(mock.starts) != 1 {
		t.Fatalf("Start must still be attempted, got %v", mock.starts)
	}
	if _, ok := st.NamespacesAccess.Selected["app"]; !ok {
		t.Fatal("failed Start must leave the namespace selected for retry")
	}
}

func TestSyncDataRelabelAwayDuringDrainCancelsPendingRestart(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	ch := make(chan k8ssync.SyncDataEvent, 8)
	mock := &mockSessions{allow: true}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}},
	})
	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.DELETED},
	})
	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}},
	})
	if len(mock.starts) != 1 {
		t.Fatalf("recreate during drain must wait for RETIRED, starts = %v", mock.starts)
	}
	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.MODIFIED, Labels: map[string]string{}},
	})
	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE_WATCH_RETIRED, Namespace: "app",
	})

	if len(mock.starts) != 1 {
		t.Fatalf("latest unmatched state must cancel restart, starts = %v", mock.starts)
	}
	if _, ok := st.NamespacesAccess.PendingLabels["app"]; ok {
		t.Fatal("latest unmatched state must clear pending labels")
	}
	if _, ok := st.NamespacesAccess.Selected["app"]; ok {
		t.Fatal("unmatched recreated namespace must not remain selected")
	}
	if _, ok := st.Namespaces["app"]; ok {
		t.Fatal("unmatched recreated namespace must not remain stored")
	}
}

func TestSyncDataDeleteAfterStartFailureRetiresNamespace(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	ch := make(chan k8ssync.SyncDataEvent, 4)
	mock := &mockSessions{allow: true, startErr: errors.New("start failed")}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}},
	})
	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.DELETED},
	})

	if _, ok := st.NamespacesAccess.Selected["app"]; ok {
		t.Fatal("deleted namespace without a session must not remain selected")
	}
	if _, ok := st.Namespaces["app"]; ok {
		t.Fatal("deleted namespace without a session must not remain stored")
	}
}

func TestSyncDataDormantEndpointsDoNotRebuildConfig(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	st.EventNamespace(nil, &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}})
	if st.Namespaces["app"].Relevant {
		t.Fatal("namespace must stay dormant until READY")
	}

	ch := make(chan k8ssync.SyncDataEvent, 4)
	mock := &mockSessions{allow: true}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType:  k8ssync.ENDPOINTS,
		Namespace: "app",
		Data:      &store.Endpoints{Namespace: "app", Service: "svc", SliceName: "s", Status: store.ADDED},
	})
	if st.Namespaces["app"].Endpoints["svc"]["s"] == nil {
		t.Fatal("dormant endpoints must still update the store")
	}
}

func TestSyncDataMatchingNamespaceDoesNotScheduleReload(t *testing.T) {
	instance.Reset()
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	ch := make(chan k8ssync.SyncDataEvent, 4)
	mock := &mockSessions{allow: true}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}},
	})
	if st.Namespaces["app"].Relevant {
		t.Fatal("matching ADD must stay dormant until READY")
	}
	// COMMAND with no HAProxy client panics if hadChanges is set.
	waitProcessed(t, ch, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
}

func TestSyncDataRetriesFailedStartOnCommand(t *testing.T) {
	instance.Reset()
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	ch := make(chan k8ssync.SyncDataEvent, 4)
	mock := &mockSessions{allow: true, startErr: errors.New("start failed")}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "app",
		Data: &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}},
	})
	if len(mock.starts) != 1 {
		t.Fatalf("initial Start attempts = %d, want 1", len(mock.starts))
	}
	waitProcessed(t, ch, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
	if len(mock.starts) != 2 {
		t.Fatalf("COMMAND must retry a failed Start, got %v", mock.starts)
	}
}

func TestSyncSelectorNamespaceUnlabelReportsChange(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	st.EventNamespace(nil, &store.Namespace{Name: "app", Status: store.ADDED, Labels: map[string]string{"watch": "true"}})
	if !st.MarkNamespaceReady("app") {
		t.Fatal("fixture must be ready so unlabel can drop it from config")
	}
	c := &HAProxyController{store: st, sessions: &mockSessions{allow: true, ready: true}}
	change := c.syncSelectorNamespace(&store.Namespace{
		Name: "app", Status: store.MODIFIED, Labels: map[string]string{},
	})
	if !change {
		t.Fatal("unlabel of a ready namespace must schedule a config rebuild")
	}
	if st.Namespaces["app"].Relevant {
		t.Fatal("unlabel must set Dormant")
	}
}

func TestSyncSelectorNamespaceMatchingLabelChangeReportsChange(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
	st.EventNamespace(nil, &store.Namespace{
		Name:   "app",
		Status: store.ADDED,
		Labels: map[string]string{"watch": "true", "routes": "denied"},
	})
	if !st.MarkNamespaceReady("app") {
		t.Fatal("fixture must be ready so label changes can affect Gateway AllowedRoutes")
	}
	c := &HAProxyController{store: st, sessions: &mockSessions{allow: true, ready: true}}
	change := c.syncSelectorNamespace(&store.Namespace{
		Name:   "app",
		Status: store.MODIFIED,
		Labels: map[string]string{"watch": "true", "routes": "allowed"},
	})
	if !change {
		t.Fatal("label change in a ready matching namespace must schedule reconciliation")
	}
	if got := st.Namespaces["app"].Labels["routes"]; got != "allowed" {
		t.Fatalf("stored routes label = %q, want allowed", got)
	}
}

func TestSyncDataAlwaysSelectsConfigMapNamespace(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{
		NamespaceLabelSelector: "watch=true",
		ConfigMap:              utils.NamespaceValue{Namespace: "base", Name: "cm"},
	})
	ch := make(chan k8ssync.SyncDataEvent, 4)
	mock := &mockSessions{allow: true, ready: true}
	c := &HAProxyController{store: st, eventChan: ch, sessions: mock}
	go c.SyncData()

	waitProcessed(t, ch, k8ssync.SyncDataEvent{
		SyncType: k8ssync.NAMESPACE, Namespace: "base",
		Data: &store.Namespace{Name: "base", Status: store.ADDED, Labels: map[string]string{}},
	})
	if len(mock.starts) != 1 || mock.starts[0] != "base" {
		t.Fatalf("configmap namespace must start a session without labels, got %v", mock.starts)
	}
	ns := st.Namespaces["base"]
	if ns == nil {
		t.Fatal("configmap namespace must be stored")
	}
	if ns.Relevant {
		t.Fatal("configmap namespace must stay non-Relevant without matching labels")
	}
}

func TestSyncSelectorNamespaceAlwaysSelectedUnlabelDropsRelevant(t *testing.T) {
	st := store.NewK8sStore(utils.OSArgs{
		NamespaceLabelSelector: "watch=true",
		ConfigMap:              utils.NamespaceValue{Namespace: "base", Name: "cm"},
	})
	mock := &mockSessions{allow: true, ready: true}
	c := &HAProxyController{store: st, sessions: mock}

	if !c.syncSelectorNamespace(&store.Namespace{
		Name: "base", Status: store.ADDED, Labels: map[string]string{"watch": "true"},
	}) {
		t.Fatal("matching labels on the configmap namespace must become Relevant")
	}
	if !st.Namespaces["base"].Relevant {
		t.Fatal("configmap namespace with matching labels must be Relevant")
	}

	if !c.syncSelectorNamespace(&store.Namespace{
		Name: "base", Status: store.MODIFIED, Labels: map[string]string{},
	}) {
		t.Fatal("unlabel of a previously Relevant configmap namespace must rebuild config")
	}
	if st.Namespaces["base"].Relevant {
		t.Fatal("unlabel must drop Relevant while keeping the watch")
	}
	if len(mock.stops) != 0 {
		t.Fatalf("unlabel must not Drain the always-selected watch, got %v", mock.stops)
	}
}
