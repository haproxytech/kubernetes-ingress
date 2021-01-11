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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_BasicAuth(t *testing.T) {
	kindURL := os.Getenv("KIND_URL")
	if kindURL == "" {
		kindURL = "127.0.0.1"
	}

	cs := k8s.New(t)
	resourceName := "basic-auth"

	var err error
	deploy := k8s.NewDeployment(resourceName)
	svc := k8s.NewService(resourceName)
	secret := k8s.NewSecret(map[string][]byte{
		// password is `password`, hashed according to method
		"md5":     []byte("$1$oGq7nTAf$rS1vV2Gu8dEGKhTvRKRoC/"),
		"des":     []byte("mskYxuygva2Ys"),
		"sha-256": []byte("$5$6If.AXwtflzSAt.v$akOQ2JHGivXo5T44bKfNEQdl.X43sGicKw5fvR4ZjN2"),
		"sha-512": []byte("$6$l39Jw4XfZOFzEJ9f$PduN9WJLBZbZz88H.M4DuT/yC2yXcXCIFor8vHafMlqkXJ0PVPW4TtZHhAMAtyexLKCwDb.o9XEzxYyljYaOS1"),
	}, resourceName)
	ing := k8s.NewIngress(resourceName, []k8s.IngressRule{{Host: resourceName, Path: "/", Service: resourceName}})
	k8s.AddAnnotations(ing, map[string]string{
		"auth-type":   "basic-auth",
		"auth-secret": secret.Name,
	})

	secret, err = cs.CoreV1().Secrets(k8s.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		t.Fail()
	}
	defer cs.CoreV1().Secrets(secret.Namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})

	deploy, err = cs.AppsV1().Deployments(k8s.Namespace).Create(context.Background(), deploy, metav1.CreateOptions{})
	if err != nil {
		t.Fail()
	}
	defer cs.AppsV1().Deployments(deploy.Namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{})

	svc, err = cs.CoreV1().Services(k8s.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
	if err != nil {
		t.Fail()
	}
	defer cs.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})

	ing, err = cs.NetworkingV1beta1().Ingresses(k8s.Namespace).Create(context.Background(), ing, metav1.CreateOptions{})
	if err != nil {
		t.Fail()
	}
	defer cs.NetworkingV1beta1().Ingresses(ing.Namespace).Delete(context.Background(), ing.Name, metav1.DeleteOptions{})

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
				dialer := &net.Dialer{}

				if addr == ing.Spec.Rules[0].Host+":80" {
					addr = kindURL + ":30080"
				}
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}

	var u *url.URL
	u, err = url.ParseRequestURI(fmt.Sprintf("http://%s/", ing.Spec.Rules[0].Host))
	if err != nil {
		t.FailNow()
	}

	req := &http.Request{
		Method: "GET",
		URL:    u,
		Host:   ing.Spec.Rules[0].Host,
		Header: map[string][]string{},
	}

	// waiting the Ingress is handled correctly
	assert.Eventually(t, func() bool {
		res, err := client.Do(req)
		if err != nil {
			t.FailNow()
		}
		defer res.Body.Close()

		return res.StatusCode == http.StatusUnauthorized
	}, waitDuration, tickDuration)

	for u, _ := range secret.Data {
		assert.Eventually(t, func() bool {
			req.SetBasicAuth(u, "password")

			res, err := client.Do(req)
			if err != nil {
				t.FailNow()
			}
			defer res.Body.Close()

			return res.StatusCode == http.StatusOK
		}, waitDuration, tickDuration)
	}
}
