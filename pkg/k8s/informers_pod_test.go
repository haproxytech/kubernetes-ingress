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

package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func controllerPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "haproxy-ingress-7d9f8b6c5-x2x4k", Namespace: "ns", UID: "uid-pod", ResourceVersion: "2"},
	}
}

func TestPodDeleteHandlerAcceptsTombstone(t *testing.T) {
	eventChan := make(chan k8ssync.SyncDataEvent, 1)
	handlers := podEventHandlers("haproxy-ingress", eventChan)

	// What client-go delivers when the watch missed the delete of the pod
	require.NotPanics(t, func() {
		handlers.DeleteFunc(cache.DeletedFinalStateUnknown{Key: "ns/haproxy-ingress-7d9f8b6c5-x2x4k", Obj: controllerPod()})
	})

	require.Len(t, eventChan, 1, "the deletion must still reach the controller")
	event := <-eventChan
	require.Equal(t, store.PodEvent{Status: store.DELETED, Name: "haproxy-ingress-7d9f8b6c5-x2x4k", Namespace: "ns"}, event.Data)
	require.Equal(t, k8ssync.POD, event.SyncType)
}

func TestPodDeleteHandlerIgnoresTombstoneWithoutPod(t *testing.T) {
	eventChan := make(chan k8ssync.SyncDataEvent, 1)
	handlers := podEventHandlers("haproxy-ingress", eventChan)

	require.NotPanics(t, func() {
		handlers.DeleteFunc(cache.DeletedFinalStateUnknown{Key: "ns/unknown"})
	})
	require.Empty(t, eventChan)
}

func TestPodDeleteHandlerSendsPodDeletion(t *testing.T) {
	eventChan := make(chan k8ssync.SyncDataEvent, 1)
	handlers := podEventHandlers("haproxy-ingress", eventChan)

	handlers.DeleteFunc(controllerPod())

	require.Len(t, eventChan, 1)
	event := <-eventChan
	require.Equal(t, store.PodEvent{Status: store.DELETED, Name: "haproxy-ingress-7d9f8b6c5-x2x4k", Namespace: "ns"}, event.Data)
}
