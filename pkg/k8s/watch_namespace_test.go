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
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// An expired watch relists a recreated namespace without observing its DELETE.
// Exercise the real informer: its store keys namespaces by name, not by UID.
func TestSelectorNamespaceUIDReplacement(t *testing.T) {
	for _, tc := range []struct {
		name        string
		oldUID      types.UID
		newUID      types.UID
		matched     bool
		terminating bool
		statuses    []store.Status
	}{
		{name: "matching replacement", oldUID: "old", newUID: "new", matched: true, statuses: []store.Status{store.DELETED, store.ADDED}},
		{name: "unmatched replacement", oldUID: "old", newUID: "new", statuses: []store.Status{store.DELETED, store.ADDED}},
		{name: "same UID unlabel", oldUID: "old", newUID: "old", statuses: []store.Status{store.MODIFIED}},
		{name: "both UIDs empty", statuses: []store.Status{store.MODIFIED}},
		{name: "old UID empty", newUID: "new", matched: true, statuses: []store.Status{store.DELETED, store.ADDED}},
		{name: "new UID empty", oldUID: "old", matched: true, statuses: []store.Status{store.DELETED, store.ADDED}},
		{name: "terminating replacement", oldUID: "old", newUID: "new", matched: true, terminating: true, statuses: []store.Status{store.DELETED, store.DELETED}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			original := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name: "app", UID: tc.oldUID, ResourceVersion: "1", Labels: map[string]string{"watch": "true"},
			}}
			replacement := *original.DeepCopy()
			replacement.UID, replacement.ResourceVersion = tc.newUID, "2"
			if !tc.matched {
				replacement.Labels = map[string]string{"watch": "false"}
			}
			if tc.terminating {
				now := metav1.Now()
				replacement.DeletionTimestamp = &now
			}
			var relist atomic.Bool
			watchStarted := make(chan struct{}, 1)
			expire := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("sendInitialEvents") == "true" {
					http.Error(w, "watch-list unsupported", http.StatusBadRequest)
					return
				}
				if r.URL.Query().Get("watch") == "true" {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
					if !relist.Load() {
						watchStarted <- struct{}{}
						select {
						case <-expire:
							_ = json.NewEncoder(w).Encode(map[string]interface{}{
								"type": "ERROR", "object": metav1.Status{
									TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"},
									Status:   "Failure", Reason: metav1.StatusReasonExpired, Code: http.StatusGone,
								},
							})
						case <-r.Context().Done():
						}
					} else {
						<-r.Context().Done()
					}
					return
				}
				ns := original
				if relist.Load() {
					ns = replacement
				}
				_ = json.NewEncoder(w).Encode(corev1.NamespaceList{
					TypeMeta: metav1.TypeMeta{Kind: "NamespaceList", APIVersion: "v1"},
					ListMeta: metav1.ListMeta{ResourceVersion: ns.ResourceVersion},
					Items:    []corev1.Namespace{ns},
				})
			}))
			defer server.Close()
			client := kubernetes.NewForConfigOrDie(&rest.Config{Host: server.URL})
			events := make(chan k8ssync.SyncDataEvent, 16)
			factory := k8sinformers.NewSharedInformerFactory(client, 0)
			k := k8s{}
			k.getSelectorNamespaceInformer(events, factory)
			stop := make(chan struct{})
			defer func() {
				close(stop)
				factory.Shutdown()
			}()
			factory.Start(stop)
			assertNamespaceEvent := func(status store.Status, labels map[string]string) {
				t.Helper()
				select {
				case ev := <-events:
					ns, ok := ev.Data.(*store.Namespace)
					if ev.SyncType != k8ssync.NAMESPACE || ev.Namespace != "app" || !ok {
						t.Fatalf("unexpected event: %#v", ev)
					}
					if ns.Status != status || ns.Name != "app" || !reflect.DeepEqual(ns.Labels, labels) {
						t.Fatalf("namespace event = %+v, want status %v, labels %v", ns, status, labels)
					}
				case <-time.After(10 * time.Second):
					t.Fatal("missing namespace event")
				}
			}
			assertNamespaceEvent(store.ADDED, original.Labels)
			select {
			case <-watchStarted:
			case <-time.After(10 * time.Second):
				t.Fatal("initial watch did not start")
			}
			relist.Store(true)
			close(expire)
			for i, status := range tc.statuses {
				labels := replacement.Labels
				if len(tc.statuses) == 2 && i == 0 {
					labels = original.Labels
				}
				assertNamespaceEvent(status, labels)
			}
		})
	}
}
