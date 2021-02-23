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

// +build e2e_sequential

package tlsauth

import (
	"crypto/tls"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type TLSAuthSuite struct {
	suite.Suite
	test            e2e.Test
	client          *e2e.Client
	validClientCert tls.Certificate
	wrongClientCert tls.Certificate
}

func (suite *TLSAuthSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.NoError(err)
	suite.client, err = e2e.NewHTTPSClient(suite.test.GetNS() + ".test")
	suite.NoError(err)
	suite.validClientCert, err = tls.LoadX509KeyPair("client-certs/valid.crt", "client-certs/valid.key")
	suite.NoError(err)
	suite.wrongClientCert, err = tls.LoadX509KeyPair("client-certs/wrong.crt", "client-certs/wrong.key")
	suite.NoError(err)
	suite.Require().NoError(suite.test.DeployYaml("config/secrets/default-cert.yaml", suite.test.GetNS()))
	suite.Require().NoError(suite.test.DeployYaml("config/client-auth.yaml", "haproxy-controller"))
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		// default backend
		return res.StatusCode == 404
	}, e2e.WaitDuration, e2e.TickDuration)
	suite.Require().NoError(suite.test.DeployYaml("config/secrets/client-ca.yaml", suite.test.GetNS()))
	suite.test.AddTearDown(func() error {
		cmd := exec.Command("kubectl", "apply", "-f", "../../config/3.configmap.yaml")
		return cmd.Run()
	})
}

func (suite *TLSAuthSuite) TearDownSuite() {
	suite.test.TearDown()
}

func TestTLSAuthSuite(t *testing.T) {
	suite.Run(t, new(TLSAuthSuite))
}
