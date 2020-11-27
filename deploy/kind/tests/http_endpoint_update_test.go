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

// +build integration

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	kindclient "github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/client"
	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_Endpoint_Update(t *testing.T) {
	cs := k8s.New(t)
	resourceName := "endpoints-update"

	var err error
	deploy := k8s.NewDeployment(resourceName)
	svc := k8s.NewService(resourceName)
	ing := k8s.NewIngress(resourceName, []k8s.IngressRule{{Host: resourceName, Path: "/", Service: resourceName}})

	deploy, err = cs.AppsV1().Deployments(k8s.Namespace).Create(context.Background(), deploy, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	defer cs.AppsV1().Deployments(deploy.Namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{})

	svc, err = cs.CoreV1().Services(k8s.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	defer cs.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})

	ing, err = cs.NetworkingV1beta1().Ingresses(k8s.Namespace).Create(context.Background(), ing, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	defer cs.NetworkingV1beta1().Ingresses(ing.Namespace).Delete(context.Background(), ing.Name, metav1.DeleteOptions{})

	type podInfoResponse struct {
		Hostname string `json:"hostname"`
	}

	// waiting the Ingress is handled correctly
	assert.Eventually(t, func() bool {
		client := kindclient.New(ing.Spec.Rules[0].Host)
		res, cls, err := client.Do("/")
		if err != nil {
			return false
		}
		defer cls()

		return res.StatusCode == http.StatusOK
	}, time.Minute, time.Second)

	for _, replicas := range []int32{2, 3, 4, 5, 4, 3, 2, 1} {
		t.Run(fmt.Sprintf("%d replicas", replicas), func(t *testing.T) {
			var s *v1.Scale
			s, err = cs.AppsV1().Deployments(deploy.Namespace).GetScale(context.TODO(), deploy.Name, metav1.GetOptions{})
			if err != nil {
				t.FailNow()
			}

			s.Spec.Replicas = replicas
			s, err = cs.AppsV1().Deployments(deploy.Namespace).UpdateScale(context.TODO(), deploy.Name, s, metav1.UpdateOptions{})
			if err != nil {
				t.FailNow()
			}

			var pl *corev1.PodList

			assert.Eventually(t, func() bool {
				pl, err = cs.CoreV1().Pods(deploy.Namespace).List(context.Background(), metav1.ListOptions{
					LabelSelector: labels.SelectorFromSet(deploy.Spec.Selector.MatchLabels).String(),
				})
				if err != nil {
					return false
				}

				return len(pl.Items) == int(replicas)
			}, time.Minute, time.Second)

			var counter int32

			registry := make(map[string]bool)
			for _, p := range pl.Items {
				registry[p.Name] = false
			}

			assert.Eventually(t, func() bool {
				client := kindclient.New(ing.Spec.Rules[0].Host)
				res, cls, err := client.Do("/")
				if err != nil {
					return false
				}
				defer cls()

				body, err := ioutil.ReadAll(res.Body)
				if err != nil {
					return false
				}

				response := &podInfoResponse{}
				if err := json.Unmarshal(body, response); err != nil {
					return false
				}

				h, ok := registry[response.Hostname]
				if ok && h == false {
					counter++
					registry[response.Hostname] = true
				}
				if !ok {
					t.Fatal("load-balancing to wrong pod")
				}

				return counter == replicas
			}, time.Minute, time.Second)
		})
	}

}
