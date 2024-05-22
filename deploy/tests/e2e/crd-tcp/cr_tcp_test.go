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
	"fmt"
	"strings"
	"testing"

	parser "github.com/haproxytech/config-parser/v5"
	"github.com/haproxytech/config-parser/v5/common"
	"github.com/haproxytech/config-parser/v5/options"
	"github.com/haproxytech/config-parser/v5/params"
	"github.com/haproxytech/config-parser/v5/types"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

// Adding GlobalSuite, just to be able to debug directly here and not from CRDTCPSuite
type TCPSuite struct {
	CRDTCPSuite
}

func TestTCPSuite(t *testing.T) {
	suite.Run(t, new(TCPSuite))
}

func (suite *TCPSuite) Test_CRD_TCP_OK() {
	suite.Run("TCP CR OK", func() {
		var err error
		suite.Require().NoError(suite.test.Apply("config/tcp-cr.yaml", suite.test.GetNS(), nil))
		suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host)
		suite.NoError(err)
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			return true
		}, e2e.WaitDuration, e2e.TickDuration)

		// Get updated config and check it
		cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
		suite.NoError(err, "Could not get Haproxy config")
		reader := strings.NewReader(cfg)
		p, err := parser.New(options.Reader(reader))
		suite.NoError(err, "Could not get Haproxy config parser")
		// Checks for tcp tcp-http-echo-443
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
		feName443 := "tcpcr_e2e-tests-crd-tcp_fe-http-echo-443"
		suite.checkFrontend(p, feName443, "bind", binds443)
		suite.checkFrontend(p, feName443, "mode", &types.StringC{Value: "tcp"})
		suite.checkFrontend(p, feName443, "log-format", &types.StringC{Value: "'%{+Q}o %t %s'"})
		suite.checkFrontend(p, feName443, "option tcplog", &types.SimpleOption{NoOption: false, Comment: ""})
		suite.checkFrontend(p, feName443, "default_backend", &types.StringC{Value: "e2e-tests-crd-tcp_http-echo_https"})

		// Check for tcp tcp-http-echo-444
		binds444 := []types.Bind{
			{
				Path: ":32767",
				Params: []params.BindOption{
					&params.BindOptionValue{
						Name:  "name",
						Value: "v4acceptproxy",
					},
					&params.BindOptionWord{
						Name: "accept-proxy",
					},
				},
			},
		}
		feName444 := "tcpcr_e2e-tests-crd-tcp_fe-http-echo-444"
		suite.checkFrontend(p, feName444, "bind", binds444)
		suite.checkFrontend(p, feName444, "mode", &types.StringC{Value: "tcp"})
		suite.checkFrontend(p, feName444, "log-format", &types.StringC{Value: "'%{+Q}o %t %s'"})
		suite.checkFrontend(p, feName444, "option tcplog", &types.SimpleOption{NoOption: false, Comment: ""})
		suite.checkFrontend(p, feName444, "default_backend", &types.StringC{Value: "e2e-tests-crd-tcp_http-echo_https2"})

		// Checks for backend
		// TODO add some checks that the backend corresponds to the deployed backend CR (already deployed, and service is arlready annotated with the backend cr)
		// TODO just need to add the proper backend configuration checks
	})
}

func (suite *TCPSuite) Test_CRD_TCP_SSL() {
	suite.Run("TCP CR SSL", func() {
		var err error
		suite.Require().NoError(suite.test.Apply("config/tcp-cr-ssl.yaml", suite.test.GetNS(), nil))
		suite.client, err = e2e.NewHTTPSClient("crdtcp-test.haproxy", 32766)
		suite.NoError(err)
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			targetCrt := res.TLS.PeerCertificates[0].Subject.CommonName == "crdtcp-test.haproxy"
			return targetCrt
		}, e2e.WaitDuration, e2e.TickDuration)
	})

	// Get updated config and check it
	cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.NoError(err, "Could not get Haproxy config")
	reader := strings.NewReader(cfg)
	p, err := parser.New(options.Reader(reader))
	suite.NoError(err, "Could not get Haproxy config parser")
	// Check for tcp tcp-http-echo-443
	binds443 := []types.Bind{
		{
			Path: ":32766",
			Params: []params.BindOption{
				&params.BindOptionValue{
					Name:  "name",
					Value: "v4",
				},
				&params.BindOptionValue{
					Name:  "crt",
					Value: "/etc/haproxy/certs/tcp/e2e-tests-crd-tcp_tcp-test-cert.pem",
				},
				&params.BindOptionWord{Name: "ssl"},
			},
		},
		{
			Path: "[::]:32766",
			Params: []params.BindOption{
				&params.BindOptionValue{
					Name:  "name",
					Value: "v4v6",
				},
				&params.BindOptionWord{Name: "v4v6"},
			},
		},
	}
	feName443 := "tcpcr_e2e-tests-crd-tcp_fe-http-echo-443"
	suite.checkFrontend(p, feName443, "bind", binds443)
	suite.checkFrontend(p, feName443, "mode", &types.StringC{Value: "tcp"})
	suite.checkFrontend(p, feName443, "log-format", &types.StringC{Value: "'%{+Q}o %t %s'"})
	suite.checkFrontend(p, feName443, "option tcplog", &types.SimpleOption{NoOption: false, Comment: ""})
	suite.checkFrontend(p, feName443, "default_backend", &types.StringC{Value: "e2e-tests-crd-tcp_http-echo_https"})
}

func (suite *TCPSuite) checkFrontend(p parser.Parser, frontendName, param string, value common.ParserData) {
	v, err := p.Get(parser.Frontends, frontendName, param)
	suite.NoError(err, "Could not get Haproxy config parser Frontend %s param %s", frontendName, param)
	suite.Equal(value, v, fmt.Sprintf("Frontend param %s should be equal to %v but is %v", param, value, v))
}
