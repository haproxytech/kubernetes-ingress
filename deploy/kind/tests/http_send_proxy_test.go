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
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kindclient "github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/client"
	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_Send_Proxy(t *testing.T) {
	var err error

	cs := k8s.New(t)
	resourceName := "send-proxy"

	deploy, svc := k8s.NewProxyProtocol(resourceName)
	svc.SetAnnotations(map[string]string{
		"send-proxy-protocol": "proxy-v1",
	})
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

	// waiting the Ingress is handled correctly
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

		return string(body) == "hello!"
	}, waitDuration, tickDuration)
}
