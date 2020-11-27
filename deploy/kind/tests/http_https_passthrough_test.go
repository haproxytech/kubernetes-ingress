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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/k8s"
)

func Test_HTTPS_Passthrough(t *testing.T) {

	kindURL := os.Getenv("KIND_URL")
	if kindURL == "" {
		kindURL = "127.0.0.1"
	}

	var err error

	cs := k8s.New(t)
	resourceName := "https-passthrough"

	deploy, svc := k8s.NewSSLDeployment(resourceName)
	ing := k8s.NewIngress(resourceName, "/")
	a := ing.GetAnnotations()
	a["haproxy.org/ssl-passthrough"] = "true"
	ing.SetAnnotations(a)

	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.FailNow()
	}
	csr := k8s.NewCertificateSigningRequest(resourceName, key, ing.Spec.Rules[0].Host)
	csr, err = cs.CertificatesV1beta1().CertificateSigningRequests().Create(context.Background(), csr, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	defer cs.CertificatesV1beta1().CertificateSigningRequests().Delete(context.Background(), csr.Name, metav1.DeleteOptions{})
	crt := k8s.ApproveCSRAndGetCertificate(t, cs, csr)
	secret := k8s.NewTLSSecret(key, crt, resourceName)
	secret, err = cs.CoreV1().Secrets(k8s.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	defer cs.CoreV1().Secrets(secret.Namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})

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
					addr = kindURL + ":30443"
				}
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}

	u, err := url.ParseRequestURI(fmt.Sprintf("https://%s/", ing.Spec.Rules[0].Host))
	if err != nil {
		t.FailNow()
	}
	req := &h.Request{
		Method: "GET",
		URL:    u,
		Host:   ing.Spec.Rules[0].Host,
	}

	assert.Eventually(t, func() bool {
		res, err := client.Do(req)
		if err != nil {
			return false
		}

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return false
		}

		return strings.HasPrefix(string(body), ing.Name)
	}, time.Minute, time.Second)
}
