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
	"io"
	"strings"
	"testing"

	parser "github.com/haproxytech/client-native/v6/config-parser"
	"github.com/haproxytech/client-native/v6/config-parser/options"
	actions "github.com/haproxytech/client-native/v6/config-parser/parsers/actions"
	tcptypes "github.com/haproxytech/client-native/v6/config-parser/parsers/tcp/types"
	"github.com/haproxytech/client-native/v6/config-parser/types"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

type TCPSuiteBackendSwitchingRule struct {
	CRDTCPSuite
}

func TestTCPSuiteBackendSwitchingRule(t *testing.T) {
	suite.Run(t, new(TCPSuiteBackendSwitchingRule))
}

// Expected haproxy configuration
// frontend tcpcr_e2e-tests-crd-tcp_fe-http-echo
//   mode tcp
//   bind 0.0.0.0:32766 name v4
//   log-format '%{+Q}o %t %s'
//   option tcplog
//   tcp-request inspect-delay 5000
//   tcp-request content accept if  { req_ssl_hello_type 1 }
//   use_backend e2e-tests-crd-tcp_http-echo-0_https if { req_ssl_sni -i backend0.example.com }
//   use_backend e2e-tests-crd-tcp_http-echo-1_https if { req_ssl_sni -i backend1.example.com }
//   default_backend e2e-tests-crd-tcp_http-echo_https

//   backend e2e-tests-crd-tcp_svc_http-echo-0_https
//   mode tcp
//   balance roundrobin
//   no option abortonclose
//   timeout server 50000
//   default-server check
//   server SRV_1 [fd00:10:244::12]:8443 enabled
//   server SRV_2 10.244.0.18:8443 enabled
//   server SRV_3 127.0.0.1:8443 disabled
//   server SRV_4 127.0.0.1:8443 disabled

// backend e2e-tests-crd-tcp_svc_http-echo-1_https
//   mode tcp
//   balance roundrobin
//   no option abortonclose
//   timeout server 50000
//   default-server check
//   server SRV_1 10.244.0.19:8443 enabled
//   server SRV_2 [fd00:10:244::13]:8443 enabled
//   server SRV_3 127.0.0.1:8443 disabled
//   server SRV_4 127.0.0.1:8443 disabled

// backend e2e-tests-crd-tcp_svc_http-echo_https
//   mode tcp
//   balance roundrobin
//   no option abortonclose
//   timeout server 50000
//   default-server check
//   server SRV_1 10.244.0.21:8443 enabled
//   server SRV_2 [fd00:10:244::15]:8443 enabled
//   server SRV_3 127.0.0.1:8443 disabled
//   server SRV_4 127.0.0.1:8443 disabled

func (suite *TCPSuiteBackendSwitchingRule) Test_CRD_TCP_BackendSwitchingRule() {
	suite.Run("TCP CR Backend Switching Rule", func() {
		var err error
		crdPath := e2e.GetCRDFixturePath() + "/tcp-cr-backend-switching-rule.yaml"
		suite.Require().NoError(suite.test.Apply(crdPath, suite.test.GetNS(), nil))

		// SNI backend0.example.com should go to http-echo-0
		suite.checkClientRequest("backend0.example.com", "http-echo-0")

		// SNI backend1.example.com should go to http-echo-1
		suite.checkClientRequest("backend1.example.com", "http-echo-1")

		// Any other SNI should go to default http-echo
		suite.client, err = e2e.NewHTTPSClient("foo.bar", 32766)
		suite.Require().NoError(err)
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			var ok bool
			if res.StatusCode == 200 {
				body, _ := io.ReadAll(res.Body)
				ok = !strings.HasPrefix(string(body), "svc_http-echo-0") && !strings.HasPrefix(string(body), "svc_http-echo-1")
			}
			return ok
		}, e2e.WaitDuration, e2e.TickDuration)

		// Get updated config and check it
		cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
		suite.Require().NoError(err, "Could not get Haproxy config")
		reader := strings.NewReader(cfg)
		p, err := parser.New(options.Reader(reader))
		suite.Require().NoError(err, "Could not get Haproxy config parser")

		feName := "tcpcr_e2e-tests-crd-tcp_fe-http-echo"
		err = suite.checkBasicHttpEchoFrontend(p, feName)
		suite.Require().NoError(err)
	})
}

// Same test as previous but switching rule
func (suite *TCPSuiteBackendSwitchingRule) Test_CRD_TCP_BackendSwitchingRule_WithAcls() {
	suite.Run("TCP CR Backend Switching Rule (with Acls)", func() {
		var err error
		crdPath := e2e.GetCRDFixturePath() + "/tcp-cr-backend-switching-rule-acls.yaml"
		suite.Require().NoError(suite.test.Apply(crdPath, suite.test.GetNS(), nil))

		// SNI backend0.example.com should go to http-echo-0
		suite.checkClientRequest("backend0.example.com", "http-echo-0")

		// SNI backend1.example.com should go to http-echo-1
		suite.checkClientRequest("backend1.example.com", "http-echo-1")

		// Any other SNI should go to default http-echo
		// suite.checkClientRequest("foo.bar", "http-echo-0")
		suite.client, err = e2e.NewHTTPSClient("foo.bar", 32766)
		suite.Require().NoError(err)
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			var ok bool
			if res.StatusCode == 200 {
				body, _ := io.ReadAll(res.Body)
				ok = !strings.HasPrefix(string(body), "http-echo-0") && !strings.HasPrefix(string(body), "http-echo-1")
			}
			return ok
		}, e2e.WaitDuration, e2e.TickDuration)

		doNotShowConfig := false
		cfg := ""
		feName := "tcpcr_e2e-tests-crd-tcp_fe-http-echo"
		var p parser.Parser
		suite.Eventually(func() bool {
			// Get updated config and check it
			cfg, err = suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
			if err != nil {
				suite.T().Logf("Could not get Haproxy config: %v", err)
				return false
			}
			reader := strings.NewReader(cfg)
			p, err = parser.New(options.Reader(reader))
			if err != nil {
				suite.T().Logf("Could not get Haproxy config parser: %v", err)
				return false
			}

			err = suite.checkBasicHttpEchoFrontend(p, feName)
			if err != nil {
				suite.T().Logf("Could not check Haproxy config in frontend %s: %v", feName, err)
				return false
			}

			// Add Acls checks
			acls := []types.ACL{
				{
					Name:      "switch_be_0",
					Criterion: "req_ssl_sni",
					Value:     "-i backend0.example.com",
				},
				{
					Name:      "switch_be_1",
					Criterion: "req_ssl_sni",
					Value:     "-i backend1.example.com",
				},
			}
			err = suite.checkFrontend(p, feName, "acl", acls)
			if err != nil {
				suite.T().Logf("Could not check Acls in Haproxy config in frontend %s: %v", feName, err)
				return false
			}
			doNotShowConfig = true
			return true
		}, e2e.WaitDuration, e2e.TickDuration, "Could not find acls in Haproxy config in frontend"+feName)

		if !doNotShowConfig {
			suite.T().Log(cfg)
		}

		// TCP Request
		tcpRequests := []types.TCPType{
			&tcptypes.InspectDelay{
				Timeout: "5000",
			},
			&tcptypes.Content{
				Action: &actions.Accept{
					Cond:     "if",
					CondTest: "{ req_ssl_hello_type 1 }",
				},
			},
		}
		err = suite.checkFrontend(p, feName, "tcp-request", tcpRequests)
		suite.Require().NoError(err)
	})
}
