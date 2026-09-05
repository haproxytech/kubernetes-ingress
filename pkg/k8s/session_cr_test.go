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

package k8s

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	crclientv1 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v1/clientset/versioned"
	crclientv3 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v3/clientset/versioned"
	crinformersv3 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v3/informers/externalversions"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

func awaitSessionTest(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for session activity")
	}
}

// Use HTTP-backed clients and the production starter, including the actual
// generated factories, handler registrations and LIST/WATCH lifecycle.
func sessionCRClient(t *testing.T, lists chan<- string) *k8s {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("sendInitialEvents") == "true" {
			http.Error(w, "watch-list unsupported", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("watch") == "true" {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		if r.URL.Path == "/apis/networking.k8s.io/v1" {
			_, _ = w.Write([]byte(`{"kind":"APIResourceList","groupVersion":"networking.k8s.io/v1","resources":[{"name":"ingresses","kind":"Ingress","namespaced":true}]}`))
			return
		}
		if !strings.Contains(r.URL.Path, "/namespaces/") {
			http.NotFound(w, r)
			return
		}
		parts := strings.Split(r.URL.Path, "/")
		resource := parts[len(parts)-1]
		if strings.Contains(r.URL.Path, "ingress.v3.haproxy.org") {
			lists <- r.URL.Path
			if strings.Contains(r.URL.Path, "/bad/") && resource == "tcps" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		kinds := map[string]string{"services": "ServiceList", "secrets": "SecretList", "ingresses": "IngressList", "endpoints": "EndpointsList", "tcps": "TCPList", "backends": "BackendList"}
		version := "v1"
		if resource == "ingresses" {
			version = "networking.k8s.io/v1"
		}
		if strings.Contains(r.URL.Path, "ingress.v3.haproxy.org") {
			version = "ingress.v3.haproxy.org/v3"
		}
		items := []interface{}{}
		if resource == "tcps" || resource == "backends" {
			items = append(items, map[string]interface{}{
				"apiVersion": version, "kind": strings.TrimSuffix(kinds[resource], "List"),
				"metadata": map[string]string{"namespace": parts[len(parts)-2], "name": "initial", "resourceVersion": "1"},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"kind": kinds[resource], "apiVersion": version, "metadata": map[string]string{"resourceVersion": "1"}, "items": items})
	}))
	t.Cleanup(server.Close)
	cfg := &rest.Config{Host: server.URL, QPS: 1000, Burst: 1000}
	k := &k8s{builtInClient: kubernetes.NewForConfigOrDie(cfg), crClientV1: crclientv1.NewForConfigOrDie(cfg), crClientV3: crclientv3.NewForConfigOrDie(cfg), crsV1: map[string]CRV1{}, crsV3: map[string]CRV3{}, crsMu: &sync.RWMutex{}}
	k.sessions = newSessionManager(make(chan k8ssync.SyncDataEvent, 100), k.startNSSession)
	t.Cleanup(k.sessions.Close)
	return k
}

type countingSessionCR struct {
	CRV3
	registrations atomic.Int32
}

func (cr *countingSessionCR) GetInformerV3(ch chan k8ssync.SyncDataEvent, factory crinformersv3.SharedInformerFactory, args utils.OSArgs) (cache.SharedIndexInformer, cache.ResourceEventHandlerRegistration) {
	cr.registrations.Add(1)
	return cr.CRV3.GetInformerV3(ch, factory, args)
}

func TestCRRegistrationDuringSessionConstruction(t *testing.T) {
	lists := make(chan string, 100)
	k := sessionCRClient(t, lists)
	constructed, release := make(chan struct{}), make(chan struct{})
	k.sessions.starter = func(ns string, epoch uint64, stop chan struct{}) (*nsSession, error) {
		sess, err := k.startNSSession(ns, epoch, stop)
		close(constructed)
		<-release
		return sess, err
	}
	started := make(chan struct{})
	go func() {
		defer close(started)
		if err := k.sessions.Start("app"); err != nil {
			t.Error(err)
		}
	}()
	awaitSessionTest(t, constructed)
	group := GroupKind{Group: "ingress.v3.haproxy.org", Kind: "TCP"}
	v1, v3 := lateCRMaps(group, utils.OSArgs{})
	cr := &countingSessionCR{CRV3: v3["TCP"]}
	v3["TCP"] = cr
	stop := make(chan struct{})
	defer close(stop)
	k.registerSessionCR(group, v1, v3, k.sessions.eventChan, stop, utils.OSArgs{})
	close(release)
	awaitSessionTest(t, started)
	select {
	case path := <-lists:
		if !strings.HasSuffix(path, "/tcps") {
			t.Fatalf("unexpected LIST %s", path)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("CR registered during construction was missed")
	}
	// Repeated discovery must not add a second registration to the factory.
	k.registerSessionCR(group, v1, v3, k.sessions.eventChan, stop, utils.OSArgs{})
	k.sessions.mu.RLock()
	sess := k.sessions.sessions["app"]
	count := len(sess.handlers)
	k.sessions.mu.RUnlock()
	if count != 5 {
		t.Fatalf("handler count = %d, want four core handlers plus TCP", count)
	}
	if got := cr.registrations.Load(); got != 1 {
		t.Fatalf("CR registrations = %d, want 1", got)
	}
}

func TestLateCRSyncDoesNotBlockNamespacesOrFutureCRDs(t *testing.T) {
	lists := make(chan string, 100)
	k := sessionCRClient(t, lists)
	// COMMAND has no namespace; distinct epochs identify its session of origin.
	k.sessions.nextEpoch["healthy"] = 10
	for _, ns := range []string{"bad", "healthy"} {
		if err := k.sessions.Start(ns); err != nil {
			t.Fatal(err)
		}
	}
	stop := make(chan struct{})
	defer close(stop)
	for _, kind := range []string{"TCP", "Backend"} {
		group := GroupKind{Group: "ingress.v3.haproxy.org", Kind: kind}
		v1, v3 := lateCRMaps(group, utils.OSArgs{})
		returned := make(chan struct{})
		go func() {
			k.registerSessionCR(group, v1, v3, k.sessions.eventChan, stop, utils.OSArgs{})
			close(returned)
		}()
		awaitSessionTest(t, returned)
		// Register Backend only after TCP's own drain. Otherwise a Backend
		// COMMAND could hide a missing or prematurely emitted TCP drain.
		seen := map[string]bool{}
		drained := map[uint64]bool{}
		wantDrains := 1
		if kind == "Backend" {
			wantDrains = 2
		}
		deadline := time.After(5 * time.Second)
		for len(drained) < wantDrains {
			select {
			case ev := <-k.sessions.eventChan:
				if ev.EventProcessed != nil {
					close(ev.EventProcessed)
				}
				switch ev.SyncType {
				case k8ssync.CR_TCP, k8ssync.CR_BACKEND:
					if string(ev.SyncType) != kind || ev.Name != "initial" {
						t.Fatalf("unexpected CR event: %+v", ev)
					}
					seen[ev.Namespace] = true
				case k8ssync.COMMAND:
					ns := "bad"
					if ev.NamespaceEpoch == 11 {
						ns = "healthy"
					} else if ev.NamespaceEpoch != 1 {
						t.Fatalf("unexpected drain epoch: %d", ev.NamespaceEpoch)
					}
					if !seen[ns] || drained[ev.NamespaceEpoch] {
						t.Fatalf("%s drain without preceding %s event (or duplicate): %+v", ns, kind, ev)
					}
					drained[ev.NamespaceEpoch] = true
				}
			case <-deadline:
				t.Fatalf("%s did not progress: events=%v drains=%v", kind, seen, drained)
			}
		}
		k.sessions.mu.RLock()
		badTCP := k.sessions.sessions["bad"].crV3.Ingress().V3().TCPs().Informer()
		k.sessions.mu.RUnlock()
		if badTCP.HasSynced() {
			t.Fatalf("bad/TCP synced despite forbidden LIST during %s progression", kind)
		}
	}
	want := map[string]bool{"bad/tcps": false, "healthy/tcps": false, "bad/backends": false, "healthy/backends": false}
	deadline := time.After(5 * time.Second)
	for remaining := len(want); remaining > 0; {
		select {
		case path := <-lists:
			for key, seen := range want {
				if !seen && strings.HasSuffix(path, "/"+key) {
					want[key] = true
					remaining--
				}
			}
		case <-deadline:
			t.Fatalf("missing LIST requests: %v", want)
		}
	}
}

func TestLateCRSyncCancellation(t *testing.T) {
	for _, name := range []string{"process", "session"} {
		t.Run(name, func(t *testing.T) {
			sess := &nsSession{stopCh: make(chan struct{})}
			stop, done, entered := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once sync.Once
			go func() {
				syncSessionCR(sess, []cache.InformerSynced{func() bool { once.Do(func() { close(entered) }); return false }}, nil, stop)
				close(done)
			}()
			awaitSessionTest(t, entered)
			if name == "process" {
				close(stop)
			} else {
				close(sess.stopCh)
			}
			awaitSessionTest(t, done)
		})
	}
}

func TestClosedSessionProxyRejectsReadyAndDrain(t *testing.T) {
	sess := &nsSession{namespace: "app", epoch: 1, stopCh: make(chan struct{}), proxy: make(chan k8ssync.SyncDataEvent)}
	m := newSessionManager(nil, nil)
	m.sessions["app"] = sess
	close(sess.stopCh)
	sess.closeProxy()
	for range 1000 {
		m.waitAndSignalReady(sess)
		if drainSessionEvents(sess, nil, nil) {
			t.Fatal("process was not stopped")
		}
	}
}

func TestSessionProxyCloseWaitsForSenders(t *testing.T) {
	for _, name := range []string{"READY", "drain"} {
		t.Run(name, func(t *testing.T) {
			sess := &nsSession{namespace: "app", epoch: 1, phase: sessionStarting, stopCh: make(chan struct{}), proxy: make(chan k8ssync.SyncDataEvent)}
			m := newSessionManager(nil, nil)
			m.sessions["app"] = sess
			var stopOnce sync.Once
			stop := func() { stopOnce.Do(func() { close(sess.stopCh) }) }
			t.Cleanup(stop)
			sent := make(chan struct{})
			go func() {
				defer close(sent)
				if name == "READY" {
					m.waitAndSignalReady(sess)
				} else if drainSessionEvents(sess, nil, nil) {
					t.Error("session cancellation reported process stop")
				}
			}()
			// No proxy receiver exists: the real sender must hold its lease
			// while blocked. Observe it without manufacturing a reader lock.
			deadline := time.After(5 * time.Second)
			tick := time.NewTicker(time.Millisecond)
			defer tick.Stop()
			for sess.proxyMu.TryLock() {
				sess.proxyMu.Unlock()
				select {
				case <-tick.C:
				case <-deadline:
					t.Fatal("blocked sender did not hold the proxy open")
				}
			}
			waitEntered, releaseWait, closed := make(chan struct{}), make(chan struct{}), make(chan struct{})
			go func() {
				closeSessionProxyAfter(func() { close(waitEntered); <-releaseWait }, sess, nil)
				close(closed)
			}()
			awaitSessionTest(t, waitEntered)
			close(releaseWait)
			select {
			case <-closed:
				t.Fatal("closed proxy with a blocked real sender")
			case <-time.After(20 * time.Millisecond):
			}
			stop()
			awaitSessionTest(t, sent)
			awaitSessionTest(t, closed)
			if _, ok := <-sess.proxy; ok {
				t.Fatal("proxy was not closed")
			}
		})
	}
}
