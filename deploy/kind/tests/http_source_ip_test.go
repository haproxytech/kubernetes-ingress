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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_Set_Source_Ip(t *testing.T) {
	type tc struct {
		HeaderName string
		IpValue    string
	}
	for _, tc := range []tc{
		{"X-Client-IP", "10.0.0.1"},
		{"X-Real-IP", "62.1.87.32"},
	} {
		t.Run(tc.HeaderName, func(t *testing.T) {
			var err error

			cs := k8s.New(t)

			cm, err := cs.CoreV1().ConfigMaps("haproxy-controller").Get(context.Background(), "haproxy-configmap", metav1.GetOptions{})
			if err != nil {
				t.FailNow()
			}
			cm.Data["src-ip-header"] = tc.HeaderName
			cm.Data["blacklist"] = tc.IpValue
			_, err = cs.CoreV1().ConfigMaps("haproxy-controller").Update(context.Background(), cm, metav1.UpdateOptions{})
			if err != nil {
				t.FailNow()
			}
			defer func() {
				cm, _ := cs.CoreV1().ConfigMaps("haproxy-controller").Get(context.Background(), "haproxy-configmap", metav1.GetOptions{})
				delete(cm.Data, "src-ip-header")
				delete(cm.Data, "blacklist")
				_, err = cs.CoreV1().ConfigMaps("haproxy-controller").Update(context.Background(), cm, metav1.UpdateOptions{})
				if err != nil {
					t.FailNow()
				}
			}()

			deploy := k8s.NewDeployment("src-ip-header", strings.ToLower(tc.HeaderName))
			svc := k8s.NewService("src-ip-header", strings.ToLower(tc.HeaderName))
			ing := k8s.NewIngress("src-ip-header", strings.ToLower(tc.HeaderName), "/")

			deploy, err = cs.AppsV1().Deployments("default").Create(context.Background(), deploy, metav1.CreateOptions{})
			assert.Nil(t, err)
			defer cs.AppsV1().Deployments(deploy.Namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{})

			svc, err = cs.CoreV1().Services("default").Create(context.Background(), svc, metav1.CreateOptions{})
			assert.Nil(t, err)
			defer cs.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})

			ing, err = cs.NetworkingV1beta1().Ingresses("default").Create(context.Background(), ing, metav1.CreateOptions{})
			assert.Nil(t, err)
			defer cs.NetworkingV1beta1().Ingresses(ing.Namespace).Delete(context.Background(), ing.Name, metav1.DeleteOptions{})

			client := &http.Client{
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
						dialer := &net.Dialer{
							Timeout:   30 * time.Second,
							KeepAlive: 30 * time.Second,
						}

						if addr == ing.Spec.Rules[0].Host+":80" {
							addr = "127.0.0.1:30080"
						}
						return dialer.DialContext(ctx, network, addr)
					},
				},
			}

			u, err := url.ParseRequestURI(fmt.Sprintf("http://%s/", ing.Spec.Rules[0].Host))
			if !assert.Nil(t, err) {
				t.FailNow()
			}
			req := &http.Request{
				Method: "GET",
				URL:    u,
				Host:   ing.Spec.Rules[0].Host,
			}

			// waiting for the Ingress to get it managed by the Ingress Controller
			assert.Eventually(t, func() bool {
				res, err := client.Do(req)
				if err != nil {
					return false
				}
				defer res.Body.Close()

				return res.StatusCode == http.StatusOK
			}, time.Minute, time.Second)

			// no headers, blacklist should ignore the rule
			for i := 0; i < 10; i++ {
				func() {
					req := &http.Request{
						Method: "GET",
						URL:    u,
						Host:   ing.Spec.Rules[0].Host,
					}
					res, err := client.Do(req)
					if err != nil {
						t.FailNow()
					}
					defer res.Body.Close()

					assert.Equal(t, http.StatusOK, res.StatusCode)
				}()
			}

			assert.Eventually(t, func() bool {
				req := &http.Request{
					Method: "GET",
					URL:    u,
					Host:   ing.Spec.Rules[0].Host,
					Header: map[string][]string{
						tc.HeaderName: {tc.IpValue},
					},
				}
				res, err := client.Do(req)
				if err != nil {
					t.FailNow()
				}
				defer res.Body.Close()

				return res.StatusCode != http.StatusOK
			}, time.Minute, time.Second)
		})
	}
}
