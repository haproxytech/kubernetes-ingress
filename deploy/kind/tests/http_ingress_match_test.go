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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kindclient "github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/client"
	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_Ingress_Match(t *testing.T) {
	resourceName := "http-ingress-match-"
	rules := []k8s.IngressRule{
		{Host: "app.haproxy", Path: "/a", Service: resourceName + "app-a"},
		{Host: "app.haproxy", Path: "/a/b", Service: resourceName + "app-b"},
		{Host: "app.haproxy", Path: "/a/b/c", Service: resourceName + "app-c"},
		{Host: "app-d.haproxy", Service: resourceName + "app-d"},
		{Path: "/e", Service: resourceName + "app-e"},
	}
	cs := k8s.New(t)
	ing := k8s.NewIngress("http-ingress-match", rules)
	ing, err := cs.NetworkingV1beta1().Ingresses(k8s.Namespace).Create(context.Background(), ing, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	defer cs.NetworkingV1beta1().Ingresses(ing.Namespace).Delete(context.Background(), ing.Name, metav1.DeleteOptions{})
	for _, rule := range rules {
		deploy := k8s.NewDeployment(rule.Service)
		svc := k8s.NewService(rule.Service)
		deploy, err := cs.AppsV1().Deployments(k8s.Namespace).Create(context.Background(), deploy, metav1.CreateOptions{})
		if err != nil {
			t.FailNow()
		}
		defer cs.AppsV1().Deployments(deploy.Namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{})
		svc, err = cs.CoreV1().Services(k8s.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
		if err != nil {
			t.FailNow()
		}
		defer cs.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})
	}
	for _, rule := range rules {
		t.Run(rule.Path, func(t *testing.T) {

			assert.Eventually(t, func() bool {
				type echoServerResponse struct {
					OS struct {
						Hostname string `json:"hostname"`
					} `json:"os"`
				}

				client := kindclient.New(rule.Host)
				res, cls, err := client.Do(rule.Path)
				if err != nil {
					return false
				}
				defer cls()

				body, err := ioutil.ReadAll(res.Body)
				if err != nil {
					return false
				}

				response := &echoServerResponse{}
				if err := json.Unmarshal(body, response); err != nil {
					return false
				}

				return strings.HasPrefix(response.OS.Hostname, rule.Service)
			}, time.Minute, time.Second)
		})
	}
}
