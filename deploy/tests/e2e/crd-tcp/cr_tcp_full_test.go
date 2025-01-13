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

	parser "github.com/haproxytech/client-native/v6/config-parser"
	"github.com/haproxytech/client-native/v6/config-parser/options"
	filtertypes "github.com/haproxytech/client-native/v6/config-parser/parsers/filters"
	tcp_actions "github.com/haproxytech/client-native/v6/config-parser/parsers/tcp/actions"
	tcptypes "github.com/haproxytech/client-native/v6/config-parser/parsers/tcp/types"
	"github.com/haproxytech/client-native/v6/config-parser/types"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

type TCPSuiteFull struct {
	CRDTCPSuite
}

func TestTCPSuiteFull(t *testing.T) {
	suite.Run(t, new(TCPSuiteFull))
}

// Expected haproxy configuration
// frontend tcpcr_e2e-tests-crd-tcp_fe-http-echo
//   mode tcp
//   bind 0.0.0.0:32766 name v4
//   acl switch_be_0 req_ssl_sni -i backend0.example.com
//   acl switch_be_1 req_ssl_sni -i backend1.example.com
//   log-format '%{+Q}o %t %s'
//   log stdout format raw daemon
//   option tcplog
//   filter trace name BEFORE-HTTP-COMP
//   filter compression
//   filter trace name AFTER-HTTP-COMP
//   tcp-request inspect-delay 5000
//   tcp-request content accept if { req_ssl_hello_type 1 }
//   use_backend e2e-tests-crd-tcp_http-echo-0_https if switch_be_0
//   use_backend e2e-tests-crd-tcp_http-echo-1_https if switch_be_1
//   default_backend e2e-tests-crd-tcp_http-echo_https
//   declare capture request len 12345
//   declare capture response len 54321

func (suite *TCPSuiteFull) Test_CRD_TCP_Full() {
	suite.Run("TCP CR Full", func() {
		crdPath := e2e.GetCRDFixturePath() + "/tcp-cr-full.yaml"
		suite.Require().NoError(suite.test.Apply(crdPath, suite.test.GetNS(), nil))

		// SNI backend0.example.com should go to http-echo-0
		suite.checkClientRequest("backend0.example.com", "http-echo-0")

		// SNI backend1.example.com should go to http-echo-1
		suite.checkClientRequest("backend1.example.com", "http-echo-1")

		// Get updated config and check it
		cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
		suite.Require().NoError(err, "Could not get Haproxy config")
		reader := strings.NewReader(cfg)
		p, err := parser.New(options.Reader(reader))
		suite.Require().NoError(err, "Could not get Haproxy config parser")

		// Check client Http Echo calls to both backend0 and backend1
		feName := "tcpcr_e2e-tests-crd-tcp_fe-http-echo"
		suite.checkBasicHttpEchoFrontend(p, feName)

		//-----------------------
		// Extra configuration checks
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
		suite.checkFrontend(p, feName, "acl", acls)

		// TCP Request
		tcpRequests := []types.TCPType{
			&tcptypes.InspectDelay{
				Timeout: "5000",
			},
			&tcptypes.Content{
				Action: &tcp_actions.Accept{
					Cond:     "if",
					CondTest: "{ req_ssl_hello_type 1 }",
				},
			},
		}
		suite.checkFrontend(p, feName, "tcp-request", tcpRequests)

		// Captures
		captures := []types.DeclareCapture{
			{
				Type:   "request",
				Length: 12345,
			},
			{
				Type:   "response",
				Length: 54321,
			},
		}
		suite.checkFrontend(p, feName, "declare capture", captures)

		// Filters
		filters := []types.Filter{
			&filtertypes.Trace{
				Name: "BEFORE-HTTP-COMP",
			},
			&filtertypes.Compression{
				Enabled: true,
			},
			&filtertypes.Trace{
				Name: "AFTER-HTTP-COMP",
			},
		}
		suite.checkFrontend(p, feName, "filter", filters)

		// Log Targets
		logTargets := []types.Log{
			{
				Address:  "stdout",
				Facility: "daemon",
				Format:   "raw",
			},
		}
		suite.checkFrontend(p, feName, "log", logTargets)
	})
}
