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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kindclient "github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/client"
	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_Http_MatchPath(t *testing.T) {
	for path, app := range map[string]string{
		"/api/v1/apps/uuid": "uuid",
		"/api/v1/apps":      "apps",
		"/api/v1":           "v1",
		"/api":              "api",
	} {
		t.Run(app, func(t *testing.T) {
			cs := k8s.New(t)
			resourceName := "ingress-match-path-" + app

			var err error
			deploy := k8s.NewDeployment(resourceName)
			svc := k8s.NewService(resourceName)
			ing := k8s.NewIngress(resourceName, []k8s.IngressRule{{Host: resourceName, Path: "/", Service: resourceName}})
			ing.Annotations["path-rewrite"] = fmt.Sprintf(`%s /\1`, path)

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

			assert.Eventually(t, func() bool {
				type podInfoResponse struct {
					Hostname string `json:"hostname"`
				}

				client := kindclient.New(ing.Spec.Rules[0].Host)
				res, cls, err := client.Do(path)
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

				return strings.HasPrefix(response.Hostname, ing.Name)
			}, time.Minute, time.Second)
		})
	}
}
