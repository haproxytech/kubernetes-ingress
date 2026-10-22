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
	"testing"
	"time"

	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
)

func TestNamespaceBootstrapBarrierWaitsForProcessing(t *testing.T) {
	t.Parallel()
	events := make(chan k8ssync.SyncDataEvent, 2)
	events <- k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE, Namespace: "app"}
	stop := make(chan struct{})
	defer close(stop)
	result := make(chan bool, 1)
	go func() { result <- waitForNamespaceEvents(events, stop) }()

	if ev := <-events; ev.SyncType != k8ssync.NAMESPACE {
		t.Fatalf("barrier overtook namespace event: %s", ev.SyncType)
	}
	var barrier k8ssync.SyncDataEvent
	select {
	case barrier = <-events:
	case <-time.After(time.Second):
		t.Fatal("bootstrap barrier was not queued")
	}
	if barrier.SyncType != k8ssync.BARRIER || barrier.EventProcessed == nil {
		t.Fatalf("expected acknowledgement-only barrier, got %#v", barrier)
	}
	select {
	case <-result:
		t.Fatal("bootstrap continued before SyncData acknowledged prior events")
	default:
	}
	close(barrier.EventProcessed)
	select {
	case ok := <-result:
		if !ok {
			t.Fatal("acknowledged bootstrap was cancelled")
		}
	case <-time.After(time.Second):
		t.Fatal("bootstrap did not continue after acknowledgement")
	}
}

func TestNamespaceBootstrapBarrierCancellation(t *testing.T) {
	for _, phase := range []string{"send", "acknowledgement"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			events := make(chan k8ssync.SyncDataEvent)
			stop := make(chan struct{})
			result := make(chan bool, 1)
			go func() { result <- waitForNamespaceEvents(events, stop) }()
			if phase == "acknowledgement" {
				select {
				case <-events:
				case <-time.After(time.Second):
					close(stop)
					t.Fatal("bootstrap barrier was not queued")
				}
			}
			close(stop)
			select {
			case ok := <-result:
				if ok {
					t.Fatal("cancelled bootstrap reported success")
				}
			case <-time.After(time.Second):
				t.Fatal("bootstrap did not stop")
			}
		})
	}
}
