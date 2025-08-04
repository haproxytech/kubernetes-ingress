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
	"github.com/haproxytech/client-native/v6/config-parser/params"
	"github.com/haproxytech/client-native/v6/config-parser/types"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

type TCPSuiteAddtionalServices struct {
	CRDTCPSuite
}

func TestTCPSuiteAddtionalServices(t *testing.T) {
	suite.Run(t, new(TCPSuiteAddtionalServices))
}

// Expected configuration:
// frontend tcpcr_e2e-tests-crd-tcp_fe-http-echo-80
//   mode tcp
//   bind :32766 name v4
//   bind [::]:32766 name v4v6 v4v6
//   log-format '%{+Q}o %t %s'
//   option tcplog
//   default_backend e2e-tests-crd-tcp_http-echo_http
// backend e2e-tests-crd-tcp_svc_http-echo-2_http ## from service/http-echo-2 (port 80)
//   mode tcp
//   balance roundrobin
//   no option abortonclose
//   timeout server 50000
//   default-server check
//   server SRV_1 10.244.0.8:8888 enabled
//   server SRV_2 [fd00:10:244::8]:8888 enabled
//   server SRV_3 127.0.0.1:8888 disabled
//   server SRV_4 127.0.0.1:8888 disabled
// backend e2e-tests-crd-tcp_svc_http-echo-2_https ## from service/http-echo-2 (port 443)
//   mode tcp
//   balance roundrobin
//   no option abortonclose
//   timeout server 50000
//   default-server check
//   server SRV_1 10.244.0.8:8443 enabled
//   server SRV_2 [fd00:10:244::8]:8443 enabled
//   server SRV_3 127.0.0.1:8443 disabled
//   server SRV_4 127.0.0.1:8443 disabled
// backend e2e-tests-crd-tcp_svc_http-echo_http ## from service/http-echo (port 80)
//   mode tcp
//   balance roundrobin
//   no option abortonclose
//   timeout server 50000
//   default-server check
//   server SRV_1 10.244.0.9:8888 enabled
//   server SRV_2 [fd00:10:244::9]:8888 enabled
//   server SRV_3 127.0.0.1:8888 disabled
//   server SRV_4 127.0.0.1:8888 disabled

func (suite *TCPSuiteAddtionalServices) Test_CRD_TCP_Additional_Services() {
	suite.Run("TCP CR Additional Services", func() {
		var err error
		crdPath := e2e.GetCRDFixturePath() + "/tcp-cr-add-services.yaml"
		suite.Require().NoError(suite.test.Apply(crdPath, suite.test.GetNS(), nil))
		client2, err := e2e.NewHTTPClient(suite.tmplData.Host2, 32766)
		suite.Require().Eventually(func() bool {
			r, cls, err := client2.Do()
			if err != nil {
				return false
			}
			defer cls()
			return r.StatusCode == 200
		}, e2e.WaitDuration, e2e.TickDuration)

		// Get updated config and check it
		cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
		suite.Require().NoError(err, "Could not get Haproxy config")
		reader := strings.NewReader(cfg)
		p, err := parser.New(options.Reader(reader))
		suite.Require().NoError(err, "Could not get Haproxy config parser")

		suite.checkFrontends(p)

		// Checks for backend
		// BE: e2e-tests-crd-tcp_svc_http-echo_http
		//  comes from TCP CR
		//  service:
		//   name: "http-echo"
		//   port: 80
		// BE e2e-tests-crd-tcp_svc_http-echo-2_http
		//  comes from TCP CR
		//  services:
		//   - name: "http-echo-2"
		//     port: 80
		// BE e2e-tests-crd-tcp_svc_http-echo-2_https"
		//  comes from TCP CR
		//  services:
		//   - name: "http-echo-2"
		//     port: 443
		beNames := []string{"e2e-tests-crd-tcp_svc_http-echo_http", "e2e-tests-crd-tcp_svc_http-echo-2_http", "e2e-tests-crd-tcp_svc_http-echo-2_https"}
		for _, beName := range beNames {
			suite.checkBackend(p, beName, "mode", &types.StringC{Value: "tcp"})
			suite.checkBackend(p, beName, "balance", &types.Balance{
				Algorithm: "roundrobin",
			})
			suite.checkBackend(p, beName, "option abortonclose", &types.SimpleOption{NoOption: true, Comment: ""})
			suite.checkBackend(p, beName, "default-server", []types.DefaultServer{
				{
					Params: []params.ServerOption{
						&params.ServerOptionWord{Name: "check"},
					},
				},
			})
		}
	})
}

func (suite *TCPSuiteAddtionalServices) checkFrontends(p parser.Parser) {
	// Checks for tcpcr_e2e-tests-crd-tcp_fe-http-echo-80
	binds443 := []types.Bind{
		{
			Path: ":32766",
			Params: []params.BindOption{
				&params.BindOptionValue{
					Name:  "name",
					Value: "v4",
				},
			},
		},
		{
			Path: "[::]:32766",
			Params: []params.BindOption{
				&params.BindOptionValue{
					Name:  "name",
					Value: "v4v6",
				},
				&params.BindOptionWord{
					Name: "v4v6",
				},
			},
		},
	}
	feName := "tcpcr_e2e-tests-crd-tcp_fe-http-echo-80"
	suite.checkFrontend(p, feName, "bind", binds443)
	suite.checkFrontend(p, feName, "mode", &types.StringC{Value: "tcp"})
	suite.checkFrontend(p, feName, "log-format", &types.StringC{Value: "'%{+Q}o %t %s'"})
	suite.checkFrontend(p, feName, "option tcplog", &types.SimpleOption{NoOption: false, Comment: ""})
	suite.checkFrontend(p, feName, "default_backend", &types.StringC{Value: "e2e-tests-crd-tcp_svc_http-echo_http"})
}
