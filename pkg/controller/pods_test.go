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
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func TestGetControllerPodPrefixResolvesDeploymentName(t *testing.T) {
	client := fake.NewSimpleClientset(
		controllerReplicaSet("default", "haproxy-ingress-abc123", "rs-1", "haproxy-ingress", "deploy-1"),
		controllerPod("default", "haproxy-ingress-abc123-aaaaa", "pod-1", "ReplicaSet", "haproxy-ingress-abc123", "rs-1"),
	)

	prefix, err := getControllerPodPrefix(context.Background(), client, "default", "haproxy-ingress-abc123-aaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if prefix != "haproxy-ingress" {
		t.Fatalf("expected Deployment name prefix, got %q", prefix)
	}
}

func TestGetControllerPodPrefixResolvesDaemonSetName(t *testing.T) {
	client := fake.NewSimpleClientset(
		controllerPod("default", "haproxy-ingress-aaaaa", "pod-1", "DaemonSet", "haproxy-ingress", "ds-1"),
	)

	prefix, err := getControllerPodPrefix(context.Background(), client, "default", "haproxy-ingress-aaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if prefix != "haproxy-ingress" {
		t.Fatalf("expected DaemonSet name prefix, got %q", prefix)
	}
}

func controllerPod(namespace, name, uid, ownerKind, ownerName, ownerUID string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			UID:       types.UID(uid),
			OwnerReferences: []metav1.OwnerReference{
				controllerOwnerReference(ownerKind, ownerName, types.UID(ownerUID)),
			},
		},
	}
}

func controllerReplicaSet(namespace, name, uid, ownerName, ownerUID string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			UID:       types.UID(uid),
			OwnerReferences: []metav1.OwnerReference{
				controllerOwnerReference("Deployment", ownerName, types.UID(ownerUID)),
			},
		},
	}
}

func controllerOwnerReference(kind, name string, uid types.UID) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         appsv1.SchemeGroupVersion.String(),
		Kind:               kind,
		Name:               name,
		UID:                uid,
		Controller:         utils.Ptr(true),
		BlockOwnerDeletion: utils.Ptr(true),
	}
}
