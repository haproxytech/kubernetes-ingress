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
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kindclient "github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/client"
	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_Rate_Limiting(t *testing.T) {
	type tc struct {
		limitPeriod      time.Duration
		limitRequests    int
		customStatusCode int
	}
	for testName, tc := range map[string]tc{
		"5s-5":        {5 * time.Second, 5, http.StatusForbidden},
		"10s-100":     {10 * time.Second, 100, http.StatusForbidden},
		"custom-code": {5 * time.Second, 1, http.StatusTooManyRequests},
	} {
		t.Run(testName, func(t *testing.T) {
			var err error

			cs := k8s.New(t)
			resourceName := "rate-limit-" + testName

			deploy := k8s.NewDeployment(resourceName)
			svc := k8s.NewService(resourceName)
			ing := k8s.NewIngress(resourceName, []k8s.IngressRule{{Host: resourceName, Path: "/", Service: resourceName}})
			k8s.AddAnnotations(ing, map[string]string{
				"rate-limit-period":      tc.limitPeriod.String(),
				"rate-limit-requests":    fmt.Sprintf("%d", tc.limitRequests),
				"rate-limit-status-code": fmt.Sprintf("%d", tc.customStatusCode),
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

			// first {limit} requests must be successful
			var counter int
			assert.Eventually(t, func() bool {
				r, cls, err := client.Do("/")
				if err != nil {
					return false
				}
				defer cls()
				counter++

				return r.StatusCode == tc.customStatusCode
			}, 15*time.Second, time.Millisecond)

			assert.Equal(t, tc.limitRequests, counter)

			// next ones should fail
			for i := 0; i < tc.limitRequests; i++ {
				func() {
					r, cls, err := client.Do("/")
					if err != nil {
						return
					}
					defer cls()
					assert.NotEqual(t, http.StatusOK, r.StatusCode)
				}()
			}

			// waiting for the rate limiting period plus extra margin
			time.Sleep(tc.limitPeriod * 2)

			r, cls, err := client.Do("/")
			if err != nil {
				t.FailNow()
			}
			defer cls()
			assert.Equal(t, 200, r.StatusCode)
		})
	}
}
