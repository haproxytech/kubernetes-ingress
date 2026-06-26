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
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func TestControllerPodMatcherMatchesDeploymentPodsAcrossReplicaSets(t *testing.T) {
	client := fake.NewSimpleClientset(
		replicaSet("default", "haproxy-ingress-abc123", "rs-1", "haproxy-ingress", "deploy-1"),
		replicaSet("default", "haproxy-ingress-def456", "rs-2", "haproxy-ingress", "deploy-1"),
		replicaSet("default", "haproxy-ingress-other", "rs-3", "haproxy-ingress-other", "deploy-2"),
		pod("default", "haproxy-ingress-abc123-aaaaa", "pod-1", "ReplicaSet", "haproxy-ingress-abc123", "rs-1"),
	)
	matcher := newControllerPodMatcher(context.Background(), client, "default", "haproxy-ingress-abc123-aaaaa", "haproxy-ingress")

	if !matcher.matches(pod("default", "haproxy-ingress-def456-bbbbb", "pod-2", "ReplicaSet", "haproxy-ingress-def456", "rs-2")) {
		t.Fatal("expected pod from another ReplicaSet owned by the same Deployment to match")
	}
	if matcher.matches(pod("default", "haproxy-ingress-other-ccccc", "pod-3", "ReplicaSet", "haproxy-ingress-other", "rs-3")) {
		t.Fatal("expected pod from another Deployment to be ignored")
	}
}

func TestControllerPodMatcherMatchesDaemonSetPodsExactly(t *testing.T) {
	client := fake.NewSimpleClientset(
		pod("default", "haproxy-ingress-aaaaa", "pod-1", "DaemonSet", "haproxy-ingress", "ds-1"),
	)
	matcher := newControllerPodMatcher(context.Background(), client, "default", "haproxy-ingress-aaaaa", "haproxy")

	if !matcher.matches(pod("default", "haproxy-ingress-bbbbb", "pod-2", "DaemonSet", "haproxy-ingress", "ds-1")) {
		t.Fatal("expected pod from the same DaemonSet to match")
	}
	if matcher.matches(pod("default", "haproxy-other-ccccc", "pod-3", "DaemonSet", "haproxy-other", "ds-2")) {
		t.Fatal("expected pod from another DaemonSet to be ignored")
	}
}

func TestControllerPodFieldSelectorMatchesAnyPod(t *testing.T) {
	selector := controllerPodFieldSelector()

	if !selector.Matches(fields.Set{"metadata.name": "haproxy-ingress-abc123-aaaaa"}) {
		t.Fatal("expected peer pod informer selector to match pod names")
	}
	if !selector.Matches(fields.Set{"spec.nodeName": "worker-1"}) {
		t.Fatal("expected peer pod informer selector to match pods without a name field")
	}
}

func pod(namespace, name, uid, ownerKind, ownerName, ownerUID string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			UID:       types.UID(uid),
			OwnerReferences: []metav1.OwnerReference{
				newControllerOwnerReference(ownerKind, ownerName, types.UID(ownerUID)),
			},
		},
	}
}

func replicaSet(namespace, name, uid, ownerName, ownerUID string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			UID:       types.UID(uid),
			OwnerReferences: []metav1.OwnerReference{
				newControllerOwnerReference("Deployment", ownerName, types.UID(ownerUID)),
			},
		},
	}
}

func newControllerOwnerReference(kind, name string, uid types.UID) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         appsv1.SchemeGroupVersion.String(),
		Kind:               kind,
		Name:               name,
		UID:                uid,
		Controller:         utils.Ptr(true),
		BlockOwnerDeletion: utils.Ptr(true),
	}
}
