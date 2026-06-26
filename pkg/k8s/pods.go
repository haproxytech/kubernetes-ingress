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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclientset "k8s.io/client-go/kubernetes"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type controllerOwner struct {
	kind string
	name string
	uid  types.UID
}

type controllerPodMatcher struct {
	client         k8sclientset.Interface
	namespace      string
	owner          controllerOwner
	fallbackPrefix string
}

func newControllerPodMatcher(ctx context.Context, client k8sclientset.Interface, namespace, podName, fallbackPrefix string) controllerPodMatcher {
	matcher := controllerPodMatcher{
		client:         client,
		namespace:      namespace,
		fallbackPrefix: fallbackPrefix,
	}
	if client == nil || namespace == "" || podName == "" {
		return matcher
	}
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		logger.Warningf("unable to resolve current pod owner for peer discovery: %v", err)
		return matcher
	}
	owner, ok := matcher.resolvePodController(ctx, pod)
	if !ok {
		return matcher
	}
	matcher.owner = owner
	return matcher
}

func (m controllerPodMatcher) enabled() bool {
	return m.owner.kind != "" || m.fallbackPrefix != ""
}

func (m controllerPodMatcher) matches(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if m.owner.kind != "" {
		owner, ok := m.resolvePodController(context.Background(), pod)
		return ok && owner == m.owner
	}
	if m.fallbackPrefix == "" {
		return false
	}
	prefix, _ := utils.GetPodPrefix(pod.Name)
	return prefix == m.fallbackPrefix
}

func (m controllerPodMatcher) resolvePodController(ctx context.Context, pod *corev1.Pod) (controllerOwner, bool) {
	ownerRef := metav1.GetControllerOf(pod)
	if ownerRef == nil {
		return controllerOwner{}, false
	}
	switch ownerRef.Kind {
	case "DaemonSet":
		return controllerOwner{kind: ownerRef.Kind, name: ownerRef.Name, uid: ownerRef.UID}, true
	case "ReplicaSet":
		return m.resolveReplicaSetOwner(ctx, ownerRef.Name)
	case "Deployment":
		return controllerOwner{kind: ownerRef.Kind, name: ownerRef.Name, uid: ownerRef.UID}, true
	default:
		return controllerOwner{}, false
	}
}

func (m controllerPodMatcher) resolveReplicaSetOwner(ctx context.Context, replicaSetName string) (controllerOwner, bool) {
	if m.client == nil || m.namespace == "" {
		return controllerOwner{}, false
	}
	replicaSet, err := m.client.AppsV1().ReplicaSets(m.namespace).Get(ctx, replicaSetName, metav1.GetOptions{})
	if err != nil {
		logger.Warningf("unable to resolve ReplicaSet owner for peer discovery: %v", err)
		return controllerOwner{}, false
	}
	ownerRef := metav1.GetControllerOf(replicaSet)
	if ownerRef == nil || ownerRef.Kind != "Deployment" {
		return controllerOwner{kind: "ReplicaSet", name: replicaSet.Name, uid: replicaSet.UID}, true
	}
	return controllerOwner{kind: ownerRef.Kind, name: ownerRef.Name, uid: ownerRef.UID}, true
}
