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

	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/http"
	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_Http_MatchPath(t *testing.T) {
	type ts struct {
		name    string
		release string
	}
	for path, ts := range map[string]ts{
		"/api/v1/apps/uuid": {name: "api", release: "uuid"},
		"/api/v1/apps":      {name: "api", release: "apps"},
		"/api/v1":           {name: "api", release: "v1"},
		"/api":              {name: "api", release: "api"},
	} {
		t.Run(path, func(t *testing.T) {
			cs := k8s.New(t)

			var err error
			deploy := k8s.NewDeployment(ts.name, ts.release)
			svc := k8s.NewService(ts.name, ts.release)
			ing := k8s.NewIngress(ts.name, ts.release, path)
			ing.Annotations["path-rewrite"] = fmt.Sprintf(`%s /\1`, path)

			deploy, err = cs.AppsV1().Deployments("default").Create(context.Background(), deploy, metav1.CreateOptions{})
			assert.Nil(t, err)
			defer cs.AppsV1().Deployments(deploy.Namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{})

			svc, err = cs.CoreV1().Services("default").Create(context.Background(), svc, metav1.CreateOptions{})
			assert.Nil(t, err)
			defer cs.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})

			ing, err = cs.NetworkingV1beta1().Ingresses("default").Create(context.Background(), ing, metav1.CreateOptions{})
			assert.Nil(t, err)
			defer cs.NetworkingV1beta1().Ingresses(ing.Namespace).Delete(context.Background(), ing.Name, metav1.DeleteOptions{})

			assert.Eventually(t, func() bool {
				type podInfoResponse struct {
					Hostname string `json:"hostname"`
				}

				client := http.New(t, ing.Spec.Rules[0].Host)
				res, c := client.Do(path)
				defer c()

				body, err := ioutil.ReadAll(res.Body)
				assert.Nil(t, err)

				response := &podInfoResponse{}
				if err := json.Unmarshal(body, response); err != nil {
					return false
				}

				return strings.HasPrefix(response.Hostname, ing.Name)
			}, time.Minute, time.Second)
		})
	}
}
