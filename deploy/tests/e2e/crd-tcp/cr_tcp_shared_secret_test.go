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

//go:build e2e_sequential

package crdtcp

import (
	"strings"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

// Test_CRD_TCP_SharedSecret covers the Secret shared by an Ingress and a TCP custom
// resource. Its certificate is written twice, once for the Ingress frontends and once for
// the bind of the custom resource, and the two copies have to coexist: skipping the second
// one left it unknown to the certificate manager, which made the cleanup pass describe it as
// a certificate to delete and reload HAProxy on every reconciliation, for nothing.
func (suite *TCPSuite) Test_CRD_TCP_SharedSecret() {
	namespace := suite.test.GetNS()
	crdPath := e2e.GetCRDFixturePath() + "/tcp-cr-ssl.yaml"
	suite.Require().NoError(suite.test.Apply(crdPath, namespace, nil))
	ingressPath := "config/ingress-shared-secret.yaml.tmpl"
	suite.Require().NoError(suite.test.Apply(ingressPath, namespace, suite.tmplData))
	suite.T().Cleanup(func() {
		suite.Require().NoError(suite.test.DeleteInNamespace(ingressPath, namespace, suite.tmplData))
	})

	tcpCert := "/etc/haproxy/certs/tcp/" + namespace + "_tcp-test-cert.pem"
	ingressCert := "/etc/haproxy/certs/frontend/" + namespace + "_tcp-test-cert.pem"

	// The certificate of the custom resource already exists when the SSL case of this
	// suite ran before, with the same Secret: the one of the Ingress is what tells that
	// the Ingress has been reconciled, so wait for both to be written.
	for _, cert := range []string{ingressCert, tcpCert} {
		suite.Eventually(func() bool {
			content, err := suite.test.GetIngressControllerFile(cert)
			if err != nil {
				suite.T().Log(err)
				return false
			}
			return strings.Contains(content, "BEGIN CERTIFICATE")
		}, e2e.WaitDuration, e2e.TickDuration)
	}

	// The bind of the custom resource names the file it was given, not the Ingress one.
	cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err, "Could not get Haproxy config")
	suite.Require().Contains(cfg, "crt "+tcpCert)
}
