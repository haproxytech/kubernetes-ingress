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

package e2e

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kindclient "github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e/client"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e/k8s"
)

func Test_Ingress_Class(t *testing.T) {
	cs := k8s.New(t)
	resourceName := "ingress-class"
	igClassName := "haproxy"

	version, _ := cs.DiscoveryClient.ServerVersion()
	major, _ := strconv.Atoi(version.Major)
	minor, _ := strconv.Atoi(version.Minor)
	if major == 1 && minor < 18 {
		t.SkipNow()
	}

	var err error
	deploy := k8s.NewDeployment(resourceName)
	svc := k8s.NewService(resourceName)
	ing := k8s.NewIngress(resourceName, []k8s.IngressRule{{Host: resourceName, Path: "/", Service: resourceName}})
	ing.Annotations = nil
	ingClass := &networkingv1beta1.IngressClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "haproxy",
		},
		Spec: networkingv1beta1.IngressClassSpec{Controller: "haproxy.org/ingress-controller"},
	}

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

	_, err = cs.NetworkingV1beta1().IngressClasses().Create(context.Background(), ingClass, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	defer cs.NetworkingV1beta1().IngressClasses().Delete(context.Background(), ingClass.Name, metav1.DeleteOptions{})

	type echoServerResponse struct {
		OS struct {
			Hostname string `json:"hostname"`
		} `json:"os"`
	}
	//
	// Ingress should be ignored: no matching ingressClass
	//
	assert.Eventually(t, func() bool {
		client := kindclient.New(ing.Spec.Rules[0].Host)
		res, cls, err := client.Do("/")
		if err != nil {
			return false
		}
		defer cls()

		return res.StatusCode == http.StatusServiceUnavailable || res.StatusCode == http.StatusNotFound
	}, waitDuration, tickDuration)

	//
	// Enabling IngressClass by updating Ingress Resources
	//
	ing.Spec.IngressClassName = &igClassName
	ing, err = cs.NetworkingV1beta1().Ingresses(k8s.Namespace).Update(context.Background(), ing, metav1.UpdateOptions{})
	if err != nil {
		t.FailNow()
	}
	assert.Eventually(t, func() bool {
		client := kindclient.New(ing.Spec.Rules[0].Host)
		res, cls, err := client.Do("/")
		if err != nil {
			return false
		}
		defer cls()

		return res.StatusCode == http.StatusOK
	}, time.Minute, time.Second)
	//
	// Removing IngressClass
	//
	err = cs.NetworkingV1beta1().IngressClasses().Delete(context.Background(), ingClass.Name, metav1.DeleteOptions{})
	if err != nil {
		t.FailNow()
	}
	assert.Eventually(t, func() bool {
		client := kindclient.New(ing.Spec.Rules[0].Host)
		res, cls, err := client.Do("/")
		if err != nil {
			return false
		}
		defer cls()

		return res.StatusCode == http.StatusServiceUnavailable || res.StatusCode == http.StatusNotFound
	}, time.Minute, time.Second)
	//
	// Reenabling IngressClass, this time by updating IngressClass instead of Ingress resource.
	//
	_, err = cs.NetworkingV1beta1().IngressClasses().Create(context.Background(), ingClass, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	defer cs.NetworkingV1beta1().IngressClasses().Delete(context.Background(), ingClass.Name, metav1.DeleteOptions{})
	assert.Eventually(t, func() bool {
		client := kindclient.New(ing.Spec.Rules[0].Host)
		res, cls, err := client.Do("/")
		if err != nil {
			return false
		}
		defer cls()

		return res.StatusCode == http.StatusOK
	}, time.Minute, time.Second)
}
