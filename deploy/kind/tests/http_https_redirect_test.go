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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	h "net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	networkngv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_Https_Redirect(t *testing.T) {
	var err error

	cs := k8s.New(t)

	deploy := k8s.NewDeployment("podinfo", "tls-redirect")
	svc := k8s.NewService("podinfo", "tls-redirect")
	ing := k8s.NewIngress("podinfo", "tls-redirect", "/")
	ing.Spec.TLS = []networkngv1beta1.IngressTLS{
		{
			Hosts:      []string{"podinfo-https.haproxy"},
			SecretName: "podinfo-https",
		},
	}

	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.FailNow()
	}
	csr := k8s.NewCertificateSigningRequest("podinfo", "tls-redirect", key, ing.Spec.TLS[0].Hosts...)
	csr, err = cs.CertificatesV1beta1().CertificateSigningRequests().Create(context.TODO(), csr, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	defer cs.CertificatesV1beta1().CertificateSigningRequests().Delete(context.Background(), csr.Name, metav1.DeleteOptions{})

	crt := k8s.ApproveCSRAndGetCertificate(t, cs, csr)

	secret := k8s.NewSecret(key, crt, "podinfo", "tls-redirect")
	secret, err = cs.CoreV1().Secrets("default").Create(context.Background(), secret, metav1.CreateOptions{})
	assert.Nil(t, err)
	defer cs.CoreV1().Secrets(secret.Namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})

	deploy, err = cs.AppsV1().Deployments("default").Create(context.Background(), deploy, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	defer cs.AppsV1().Deployments(deploy.Namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{})

	svc, err = cs.CoreV1().Services("default").Create(context.Background(), svc, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	defer cs.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})

	ing, err = cs.NetworkingV1beta1().Ingresses("default").Create(context.Background(), ing, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	defer cs.NetworkingV1beta1().Ingresses(ing.Namespace).Delete(context.Background(), ing.Name, metav1.DeleteOptions{})

	ca := k8s.GetCaOrFail(t, cs)

	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(ca)

	client := &h.Client{
		CheckRedirect: func(req *h.Request, via []*h.Request) error {
			return h.ErrUseLastResponse
		},
		Transport: &h.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
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
	req := &h.Request{
		Method: "GET",
		URL:    u,
		Host:   ing.Spec.Rules[0].Host,
	}

	a := ing.GetAnnotations()

	copyAnnotations := func(src map[string]string) (dst map[string]string) {
		dst = make(map[string]string)
		for k, v := range src {
			dst[k] = v
		}
		return
	}


	t.Run("enabled", func(t *testing.T) {
		e := copyAnnotations(a)
		e["ssl-redirect"] = "true"
		ing.SetAnnotations(e)
		ing, err = cs.NetworkingV1beta1().Ingresses(ing.Namespace).Update(context.Background(), ing, metav1.UpdateOptions{})
		if err != nil {
			t.FailNow()
		}
		assert.Eventually(t, func() bool {
			res, err := client.Do(req)
			if err != nil {
				return false
			}
			defer res.Body.Close()

			return res.StatusCode == 302
		}, time.Minute, time.Second)
	})
	t.Run("disabled", func(t *testing.T) {
		e := copyAnnotations(a)
		e["ssl-redirect"] = "false"
		ing.SetAnnotations(e)
		ing, err = cs.NetworkingV1beta1().Ingresses(ing.Namespace).Update(context.Background(), ing, metav1.UpdateOptions{})
		if err != nil {
			t.FailNow()
		}
		assert.Eventually(t, func() bool {
			res, err := client.Do(req)
			if err != nil {
				return false
			}
			defer res.Body.Close()

			return res.StatusCode < 300
		}, time.Minute, time.Second)
	})
	t.Run("enabled with custom code", func(t *testing.T) {
		e := copyAnnotations(a)
		e["ssl-redirect"] = "true"
		e["ssl-redirect-code"] = "301"
		ing.SetAnnotations(e)
		ing, err = cs.NetworkingV1beta1().Ingresses(ing.Namespace).Update(context.Background(), ing, metav1.UpdateOptions{})
		if err != nil {
			t.FailNow()
		}
		assert.Eventually(t, func() bool {
			res, err := client.Do(req)
			if err != nil {
				return false
			}
			defer res.Body.Close()

			return res.StatusCode == 301
		}, time.Minute, time.Second)
	})
}
