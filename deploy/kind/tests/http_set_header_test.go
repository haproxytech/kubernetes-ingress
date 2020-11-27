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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kindclient "github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/client"
	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_Request_Set_Header(t *testing.T) {
	type tc struct {
		name  string
		value string
	}
	for header, tc := range map[string]tc{
		"cache-control":   {"Cache-Control", "no-store,no-cache,private"},
		"x-custom-header": {"X-Custom-Header", "haproxy-ingress-controller"},
	} {
		t.Run(header, func(t *testing.T) {
			var err error

			cs := k8s.New(t)
			resourceName := "http-req-set-header-" + header

			deploy := k8s.NewDeployment(resourceName)
			k8s.EditPodImage(deploy, "ealen/echo-server:latest")
			k8s.EditPodCommand(deploy)
			k8s.EditPodExposedPort(deploy, 80)
			deploy, err = cs.AppsV1().Deployments(k8s.Namespace).Create(context.Background(), deploy, metav1.CreateOptions{})
			if err != nil {
				t.FailNow()
			}
			defer cs.AppsV1().Deployments(deploy.Namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{})

			svc := k8s.NewService(resourceName)
			k8s.EditServicePort(svc, 80)
			svc, err = cs.CoreV1().Services(k8s.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
			if err != nil {
				t.FailNow()
			}
			defer cs.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})

			ing := k8s.NewIngress(resourceName, "/")
			k8s.AddAnnotations(ing, map[string]string{
				"request-set-header": fmt.Sprintf("%s %q", tc.name, tc.value),
			})
			ing, err = cs.NetworkingV1beta1().Ingresses(k8s.Namespace).Create(context.Background(), ing, metav1.CreateOptions{})
			if err != nil {
				t.FailNow()
			}
			defer cs.NetworkingV1beta1().Ingresses(ing.Namespace).Delete(context.Background(), ing.Name, metav1.DeleteOptions{})

			client := kindclient.NewClient(ing.Spec.Rules[0].Host, 30080)

			assert.Eventually(t, func() bool {
				r, cls, err := client.Do("/")
				if err != nil {
					return false
				}
				defer cls()
				return r.StatusCode == http.StatusOK
			}, time.Minute, time.Second)

			assert.Eventually(t, func() bool {
				r, cls, err := client.Do("/")
				if err != nil {
					return false
				}
				defer cls()

				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					return false
				}

				type echo struct {
					Request struct {
						Headers map[string]string `json:"headers"`
					} `json:"request"`
				}

				e := &echo{}

				if err := json.Unmarshal(b, e); err != nil {
					return false
				}

				v, ok := e.Request.Headers[strings.ToLower(tc.name)]

				if !ok {
					return false
				}

				return v == tc.value
			}, time.Minute, time.Second)
		})
	}
}

func Test_Response_Set_Header(t *testing.T) {
	type tc struct {
		name  string
		value string
	}
	for header, tc := range map[string]tc{
		"cache-control":   {"Cache-Control", "no-store,no-cache,private"},
		"x-custom-header": {"X-Custom-Header", "haproxy-ingress-controller"},
	} {
		t.Run(header, func(t *testing.T) {
			var err error
			cs := k8s.New(t)
			resourceName := "http-res-set-header" + header

			deploy := k8s.NewDeployment(resourceName)
			svc := k8s.NewService(resourceName)
			ing := k8s.NewIngress(resourceName, "/")
			k8s.AddAnnotations(ing, map[string]string{
				"response-set-header": fmt.Sprintf("%s %q", strings.ToLower(tc.name), tc.value),
			})

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

			client := kindclient.NewClient(ing.Spec.Rules[0].Host, 30080)

			assert.Eventually(t, func() bool {
				r, cls, err := client.Do("/")
				if err != nil {
					return false
				}
				defer cls()
				return r.StatusCode == http.StatusOK
			}, time.Minute, time.Second)

			assert.Eventually(t, func() bool {
				r, cls, err := client.Do("/")
				if err != nil {
					return false
				}
				defer cls()

				return r.Header.Get(tc.name) == tc.value
			}, time.Minute, time.Second)
		})
	}
}
