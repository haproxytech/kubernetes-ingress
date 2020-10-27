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

func Test_Set_Host(t *testing.T) {
	for _, host := range []string{"host.haproxy", "foo.haproxy", "bar.tld"} {
		t.Run(host, func(t *testing.T) {
			name := strings.Replace(host, ".", "-", -1)
			var err error

			cs := k8s.New(t)

			deploy := k8s.NewDeployment("http-echo", name)
			k8s.EditPodImage(deploy, "ealen/echo-server:latest")
			k8s.EditPodCommand(deploy)
			k8s.EditPodExposedPort(deploy, 80)
			deploy, err = cs.AppsV1().Deployments("default").Create(context.Background(), deploy, metav1.CreateOptions{})
			assert.Nil(t, err)
			defer cs.AppsV1().Deployments(deploy.Namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{})

			svc := k8s.NewService("http-echo", name)
			k8s.EditServicePort(svc, 80)
			svc, err = cs.CoreV1().Services("default").Create(context.Background(), svc, metav1.CreateOptions{})
			assert.Nil(t, err)
			defer cs.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})

			ing := k8s.NewIngress("http-echo", name, "/")
			k8s.AddAnnotations(ing, map[string]string{
				"set-host": host,
			})
			ing, err = cs.NetworkingV1beta1().Ingresses("default").Create(context.Background(), ing, metav1.CreateOptions{})
			assert.Nil(t, err)
			defer cs.NetworkingV1beta1().Ingresses(ing.Namespace).Delete(context.Background(), ing.Name, metav1.DeleteOptions{})

			client := kindclient.NewClient(t, ing.Spec.Rules[0].Host, 30080)

			assert.Eventually(t, func() bool {
				r, cls := client.Do("/")
				defer cls()
				return r.StatusCode == http.StatusOK
			}, time.Minute, time.Second)

			assert.Eventually(t, func() bool {
				r, cls := client.Do("/")
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

				v, ok := e.Request.Headers["host"]

				if !ok {
					return false
				}

				return v == host
			}, time.Minute, time.Second)
		})
	}
}
