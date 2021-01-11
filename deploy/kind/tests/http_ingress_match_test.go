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
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kindclient "github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/client"
	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_Ingress_Match(t *testing.T) {
	resourceName := "http-ingress-match-"
	rules := []k8s.IngressRule{
		{Host: "app.haproxy", Path: "/", Service: resourceName + "app-0"},
		{Host: "app.haproxy", Path: "/a", Service: resourceName + "app-1"},
		{Host: "app.haproxy", Path: "/a/b", Service: resourceName + "app-2"},
		{Host: "app.haproxy", Path: "/exact", PathType: "Exact", Service: resourceName + "app-3"},
		{Host: "app.haproxy", Path: "/exactslash/", PathType: "Exact", Service: resourceName + "app-4"},
		{Host: "app.haproxy", Path: "/prefix", PathType: "Prefix", Service: resourceName + "app-5"},
		{Host: "app.haproxy", Path: "/prefixslash/", PathType: "Prefix", Service: resourceName + "app-6"},
		{Host: "sub.app.haproxy", Service: resourceName + "app-7"},
		{Host: "*.haproxy", Service: resourceName + "app-8"},
		{Path: "/test", Service: resourceName + "app-9"},
	}
	type test struct {
		target string
		host   string
		paths  []string
	}
	// For each test, requests made to "paths" should be
	// answered by the ingressRule.Service of that same test.
	// Ref: https://kubernetes.io/docs/concepts/services-networking/ingress/#path-types
	tests := []test{
		{target: "app-0", host: "app.haproxy", paths: []string{"/", "/test", "/exact/", "/exactslash", "/exactslash/foo", "/prefixxx"}},
		{target: "app-1", host: "app.haproxy", paths: []string{"/a"}},
		{target: "app-2", host: "app.haproxy", paths: []string{"/a/b"}},
		{target: "app-3", host: "app.haproxy", paths: []string{"/exact"}},
		{target: "app-4", host: "app.haproxy", paths: []string{"/exactslash/"}},
		{target: "app-5", host: "app.haproxy", paths: []string{"/prefix", "/prefix/", "/prefix/foo"}},
		{target: "app-6", host: "app.haproxy", paths: []string{"/prefixslash", "/prefixslash/", "/prefixslash/foo/bar"}},
		{target: "app-7", host: "sub.app.haproxy", paths: []string{"/test"}},
		{target: "app-8", host: "test.haproxy", paths: []string{"/test"}},
		{target: "app-9", host: "foo.bar", paths: []string{"/test"}},
	}

	cs := k8s.New(t)
	version, _ := cs.DiscoveryClient.ServerVersion()
	major, _ := strconv.Atoi(version.Major)
	minor, _ := strconv.Atoi(version.Minor)
	if major == 1 && minor < 18 {
		tests[0] = test{target: "app-0", host: "app.haproxy", paths: []string{"/", "/test", "/prefixxx"}}
	}
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
	for _, test := range tests {
		for _, path := range test.paths {
			t.Run(test.host+path, func(t *testing.T) {

				assert.Eventually(t, func() bool {
					type echoServerResponse struct {
						OS struct {
							Hostname string `json:"hostname"`
						} `json:"os"`
					}

					client := kindclient.New(test.host)
					res, cls, err := client.Do(path)
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

					//t.Logf("%s --> %s", path, response.OS.Hostname)
					return strings.HasPrefix(response.OS.Hostname, resourceName+test.target)
				}, waitDuration, tickDuration)
			})
		}
	}
}
