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

package httpsruntime

import (
	"net/http"
	"testing"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

// Adding OffloadRuntimeSuite, just to be able to debug directly here and not from CRDSuite
type OffloadRuntimeSuite struct {
	HTTPSSuite
}

func TestOffloadRuntimeSuite(t *testing.T) {
	suite.Run(t, new(OffloadRuntimeSuite))
}

func (suite *OffloadRuntimeSuite) Test_HTTS_OffloadRuntime() {
	suite.tmplData.Port = "https"

	var err error
	// Deploy all secrets
	suite.Require().NoError(suite.test.Apply("config/secret-default.yaml", suite.test.GetNS(), nil))
	suite.Require().NoError(suite.test.Apply("config/secret-offload.yaml", suite.test.GetNS(), nil))
	suite.Require().NoError(suite.test.Apply("config/secret-offload-1.yaml", suite.test.GetNS(), nil))
	suite.Require().NoError(suite.test.Apply("config/secret-offload-2.yaml", suite.test.GetNS(), nil))
	suite.Require().NoError(suite.test.Apply("config/secret-offload-3.yaml", suite.test.GetNS(), nil))
	suite.Require().NoError(suite.test.Apply("config/secret-offload-4.yaml", suite.test.GetNS(), nil))

	// Deploy all echo-app with tls.host default.haproxy
	suite.Require().NoError(suite.test.Apply("config/echo-app-offload-default.yaml", suite.test.GetNS(), nil))
	suite.client, err = e2e.NewHTTPSClient("offload-test.haproxy", 0)
	suite.Require().NoError(err)
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil || err != nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		targetCrt := res.TLS.PeerCertificates[0].Subject.CommonName == "default.haproxy"
		return targetCrt && res.StatusCode == http.StatusOK
	}, e2e.WaitDuration, e2e.TickDuration)

	// Now deploy the echo-app with tls.host:
	// - offload-test.haproxy
	// - offload-test-1.haproxy
	// - offload-test-2.haproxy
	// - offload-test-3.haproxy
	// - offload-test-4.haproxy
	// - default.haproxy
	// Preparation to check that no reload occurs

	oldInfo, err := e2e.GetGlobalHAProxyInfo()
	suite.Require().NoError(err)

	suite.Require().NoError(suite.test.Apply("config/echo-app-offload.yaml", suite.test.GetNS(), nil))
	suite.client, err = e2e.NewHTTPSClient("offload-test.haproxy", 0)
	suite.Require().NoError(err)
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		targetCrt := res.TLS.PeerCertificates[0].Subject.CommonName == "offload-test.haproxy"
		return targetCrt && res.StatusCode == http.StatusOK
	}, e2e.WaitDuration, e2e.TickDuration)

	// 	Check that no reload occurs !!!!!
	newInfo, err := e2e.GetGlobalHAProxyInfo()
	suite.Require().NoError(err)
	suite.Require().Equal(oldInfo.Pid, newInfo.Pid)

	// Now change secret haproxy-offload-test content and put the content from haproxy-offload-test-4
	suite.Require().NoError(suite.test.Apply("config/secret-offload-with-4-content.yaml", suite.test.GetNS(), nil))

	suite.client, err = e2e.NewHTTPSClient("offload-test.haproxy", 0)
	suite.Require().NoError(err)
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		targetCrt := res.TLS.PeerCertificates[0].Subject.CommonName == "default.haproxy"
		return targetCrt && res.StatusCode == http.StatusOK
	}, e2e.WaitDuration, e2e.TickDuration)

	// 	Check that no reload occurs !!!!!
	newInfo2, err := e2e.GetGlobalHAProxyInfo()
	suite.Require().NoError(err)
	suite.Require().Equal(newInfo.Pid, newInfo2.Pid)

	//------
	// Check that cert contains offload-test-4.haproxy through runtime
	certInfo, certErr := e2e.GetCertSubject("/etc/haproxy/certs/frontend/e2e-tests-https-runtime_haproxy-offload-test.pem")
	suite.Require().NoError(certErr)
	suite.Require().Containsf(certInfo.Subject, "offload-test-4.haproxy", "wrong Cert CN. got [%s], expected [%s]", certInfo.Subject, "offload-test-4.haproxy")

	// Now trigger a reload
	suite.Require().NoError(suite.test.Apply("config/backend-crd.yaml", suite.test.GetNS(), nil))
	suite.Require().NoError(suite.test.Apply("config/echo-app-offload-backend-crd.yaml", suite.test.GetNS(), nil))

	suite.Eventually(func() bool {
		newInfo3, err := e2e.GetGlobalHAProxyInfo()
		suite.T().Log(err)
		if err != nil {
			return false
		}
		return newInfo3.Pid != newInfo2.Pid
	}, e2e.WaitDuration, e2e.TickDuration)

	//------
	// Check that cert contains offload-test-4.haproxy through runtime
	certInfo2, certErr2 := e2e.GetCertSubject("/etc/haproxy/certs/frontend/e2e-tests-https-runtime_haproxy-offload-test.pem")
	suite.Require().NoError(certErr2)
	suite.Require().Containsf(certInfo2.Subject, "offload-test-4.haproxy", "wrong Cert CN. got [%s], expected [%s]", certInfo2.Subject, "offload-test-4.haproxy")
}
