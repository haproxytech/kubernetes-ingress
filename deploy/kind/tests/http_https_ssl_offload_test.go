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
	"io/ioutil"
	"net"
	h "net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	networkngv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_Https_Ssl_Offload(t *testing.T) {
	var err error

	cs := k8s.New(t)

	deploy, svc := k8s.NewOffloadedSsl("simple-https-listener", "ssl")

	svc, err = cs.CoreV1().Services("default").Create(context.Background(), svc, metav1.CreateOptions{})
	assert.Nil(t, err)
	defer cs.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})

	ing := k8s.NewIngress("simple-https-listener", "ssl", "/")
	ing.Spec.TLS = []networkngv1beta1.IngressTLS{
		{
			Hosts:      []string{"simple-https-listener-ssl.haproxy"},
			SecretName: "simple-https-listener-ssl",
		},
	}
	a := ing.GetAnnotations()
	a["haproxy.org/ssl-passthrough"] = "true"
	ing.SetAnnotations(a)
	ing, err = cs.NetworkingV1beta1().Ingresses("default").Create(context.Background(), ing, metav1.CreateOptions{})
	assert.Nil(t, err)
	defer cs.NetworkingV1beta1().Ingresses(ing.Namespace).Delete(context.Background(), ing.Name, metav1.DeleteOptions{})

	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if !assert.Nil(t, err) {
		t.FailNow()
	}
	csr := k8s.NewCertificateSigningRequest("simple-https-listener", "ssl", key, ing.Spec.TLS[0].Hosts...)
	csr, err = cs.CertificatesV1beta1().CertificateSigningRequests().Create(context.Background(), csr, metav1.CreateOptions{})
	if !assert.Nil(t, err) {
		t.FailNow()
	}
	defer cs.CertificatesV1beta1().CertificateSigningRequests().Delete(context.Background(), csr.Name, metav1.DeleteOptions{})

	crt := k8s.ApproveCSRAndGetCertificate(t, cs, csr)

	secret := k8s.NewSecret(key, crt, "simple-https-listener", "ssl")
	secret, err = cs.CoreV1().Secrets("default").Create(context.Background(), secret, metav1.CreateOptions{})
	assert.Nil(t, err)
	defer cs.CoreV1().Secrets(secret.Namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})

	deploy, err = cs.AppsV1().Deployments("default").Create(context.Background(), deploy, metav1.CreateOptions{})
	assert.Nil(t, err)
	defer cs.AppsV1().Deployments(deploy.Namespace).Delete(context.Background(), deploy.Name, metav1.DeleteOptions{})

	caCertPool := x509.NewCertPool()
	ca := k8s.GetCaOrFail(t, cs)
	caCertPool.AddCert(ca)

	client := &h.Client{
		Transport: &h.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
			DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
				dialer := &net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}

				if addr == ing.Spec.Rules[0].Host+":443" {
					addr = "127.0.0.1:30443"
				}
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}

	u, err := url.ParseRequestURI(fmt.Sprintf("https://%s/", ing.Spec.Rules[0].Host))
	if !assert.Nil(t, err) {
		t.FailNow()
	}
	req := &h.Request{
		Method: "GET",
		URL:    u,
		Host:   ing.Spec.Rules[0].Host,
	}

	assert.Eventually(t, func() bool {
		res, err := client.Do(req)
		if !assert.Nil(t, err) {
			return false
		}

		body, err := ioutil.ReadAll(res.Body)
		if !assert.Nil(t, err) {
			return false
		}

		return strings.HasPrefix(string(body), ing.Name)
	}, time.Minute, 10*time.Second)
}
