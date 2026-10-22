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

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func getControllerPodPrefix(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) (string, error) {
	fallbackPrefix, fallbackErr := utils.GetPodPrefix(podName)
	if clientset == nil || namespace == "" || podName == "" {
		return fallbackPrefix, fallbackErr
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fallbackPrefix, err
	}
	ownerRef := metav1.GetControllerOf(pod)
	if ownerRef == nil {
		return fallbackPrefix, fallbackErr
	}

	switch ownerRef.Kind {
	case "DaemonSet", "Deployment":
		return ownerRef.Name, nil
	case "ReplicaSet":
		replicaSet, err := clientset.AppsV1().ReplicaSets(namespace).Get(ctx, ownerRef.Name, metav1.GetOptions{})
		if err != nil {
			return fallbackPrefix, err
		}
		replicaSetOwnerRef := metav1.GetControllerOf(replicaSet)
		if replicaSetOwnerRef == nil || replicaSetOwnerRef.Kind != "Deployment" {
			return ownerRef.Name, nil
		}
		return replicaSetOwnerRef.Name, nil
	default:
		return fallbackPrefix, fallbackErr
	}
}
