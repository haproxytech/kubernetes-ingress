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
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kindclient "github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/client"
	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_Http_Send_Proxy(t *testing.T) {
	var err error

	cs := k8s.New(t)

	deploy, svc := k8s.NewProxyProtocol("proxy", "v1")
	svc.SetAnnotations(map[string]string{
		"send-proxy-protocol": "proxy-v1",
	})
	ing := k8s.NewIngress("proxy", "v1", "/")

	deploy, err = cs.AppsV1().Deployments("default").Create(context.Background(), deploy, metav1.CreateOptions{})
	assert.Nil(t, err)
	defer cs.AppsV1().Deployments(deploy.Namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{})

	svc, err = cs.CoreV1().Services("default").Create(context.Background(), svc, metav1.CreateOptions{})
	assert.Nil(t, err)
	defer cs.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})

	ing, err = cs.NetworkingV1beta1().Ingresses("default").Create(context.Background(), ing, metav1.CreateOptions{})
	assert.Nil(t, err)
	defer cs.NetworkingV1beta1().Ingresses(ing.Namespace).Delete(context.Background(), ing.Name, metav1.DeleteOptions{})

	// waiting the Ingress is handled correctly
	assert.Eventually(t, func() bool {
		client := kindclient.New(t, ing.Spec.Rules[0].Host)
		res, c := client.Do("/")
		defer c()

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return false
		}

		return string(body) == "hello!"
	}, time.Minute, time.Second)
}
