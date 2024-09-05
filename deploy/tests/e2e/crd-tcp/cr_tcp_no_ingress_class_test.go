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
	"testing"

	parser "github.com/haproxytech/config-parser/v5"
	"github.com/haproxytech/config-parser/v5/options"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

type TCPSuiteNoIngressClass struct {
	CRDTCPSuite
}

func TestTCPSuiteNoIngressClasss(t *testing.T) {
	suite.Run(t, new(TCPSuiteNoIngressClass))
}

// Expected configuration:
// frontend tcpcr_e2e-tests-crd-tcp_fe-http-echo-80
// backend e2e-tests-crd-tcp_http-echo-2_http ## from service/http-echo-2 (port 80)
// backend e2e-tests-crd-tcp_http-echo-2_https ## from service/http-echo-2 (port 443)
// backend e2e-tests-crd-tcp_http-echo_http ## from service/http-echo (port 80)
// SHOULD NOT be created

func (suite *TCPSuiteNoIngressClass) Test_CRD_TCP_No_Ingress_Class() {
	suite.Run("TCP CR Additional Services", func() {
		var err error
		suite.Require().NoError(suite.test.Apply("config/tcp-cr-no-ingress-class.yaml", suite.test.GetNS(), nil))
		client2, err := e2e.NewHTTPClient(suite.tmplData.Host2, 32766)
		suite.Require().Eventually(func() bool {
			_, _, err := client2.Do()
			return err != nil // should return an error!
		}, e2e.WaitDuration, e2e.TickDuration)

		// Get updated config and check it
		cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
		suite.NoError(err, "Could not get Haproxy config")
		reader := strings.NewReader(cfg)
		p, err := parser.New(options.Reader(reader))
		suite.NoError(err, "Could not get Haproxy config parser")

		_, err = p.Get(parser.Frontends, "tcpcr_e2e-tests-crd-tcp_fe-http-echo-80", "bind")
		suite.Require().Equal(err.Error(), "section missing")
		_, err = p.Get(parser.Backends, "e2e-tests-crd-tcp_http-echo-2_http", "mode")
		suite.Require().Equal(err.Error(), "section missing")
		_, err = p.Get(parser.Backends, "e2e-tests-crd-tcp_http-echo-2_https", "mode")
		suite.Require().Equal(err.Error(), "section missing")
		_, err = p.Get(parser.Backends, "e2e-tests-crd-tcp_http-echo_http", "mode")
		suite.Require().Equal(err.Error(), "section missing")
	})
}
