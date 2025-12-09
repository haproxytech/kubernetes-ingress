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

// Adding TCPSuite, just to be able to debug directly here and not from CRDTCPSuite
type TCPSuite struct {
	CRDTCPSuite
}

func TestTCPSuite(t *testing.T) {
	suite.Run(t, new(TCPSuite))
}

func (suite *TCPSuite) Test_CRD_TCP_OK() {
	suite.Run("TCP CR OK", func() {
		var err error
		crdPath := e2e.GetCRDFixturePath() + "/tcp-cr.yaml"
		suite.Require().NoError(suite.test.Apply(crdPath, suite.test.GetNS(), nil))
		suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host, 32766)
		suite.Require().NoError(err)
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			return true
		}, e2e.WaitDuration, e2e.TickDuration)

		var p parser.Parser
		suite.Eventually(func() bool {
			// Get updated config and check it
			cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
			if err != nil {
				suite.T().Log(err)
				return false
			}
			reader := strings.NewReader(cfg)
			p, err = parser.New(options.Reader(reader))
			if err != nil {
				suite.T().Log(err)
				return false
			}

			err = suite.checkFrontends(p)
			if err != nil {
				suite.T().Log(err)
				return false
			}

			// Checks for backend
			// Checks that we have correctly applied the backend CR
			beNames := []string{"e2e-tests-crd-tcp_svc_http-echo_http", "e2e-tests-crd-tcp_svc_http-echo_http2"}
			for _, beName := range beNames {
				err = suite.checkBackend(p, beName, "mode", &types.StringC{Value: "tcp"})
				if err != nil {
					suite.T().Log(err)
					return false
				}
				err = suite.checkBackend(p, beName, "balance", &types.Balance{
					Algorithm: "roundrobin",
				})
				if err != nil {
					suite.T().Log(err)
					return false
				}
				err = suite.checkBackend(p, beName, "option abortonclose", &types.SimpleOption{NoOption: true, Comment: ""})
				if err != nil {
					suite.T().Log(err)
					return false
				}
				err = suite.checkBackend(p, beName, "default-server", []types.DefaultServer{
					{
						Params: []params.ServerOption{
							&params.ServerOptionWord{Name: "check"},
						},
					},
				})
				if err != nil {
					suite.T().Log(err)
					return false
				}
			}

			return true
		}, e2e.WaitDuration, e2e.TickDuration)
	})
}

func (suite *TCPSuite) Test_CRD_TCP_CR_Backend() {
	suite.Run("TCP CR with Backend CR", func() {
		var err error
		crdPath := e2e.GetCRDFixturePath() + "/tcp-cr.yaml"
		suite.Require().NoError(suite.test.Apply(crdPath, suite.test.GetNS(), nil))
		suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host, 32766)
		suite.Require().NoError(err)
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
		suite.Require().NoError(err, "Could not get Haproxy config")
		reader := strings.NewReader(cfg)
		p, err := parser.New(options.Reader(reader))
		suite.Require().NoError(err, "Could not get Haproxy config parser")

		err = suite.checkFrontends(p)
		suite.Require().NoError(err)
		// Checks for backend e2e-tests-crd-tcp_svc_http-echo_http
		beNames := []string{"e2e-tests-crd-tcp_svc_http-echo_http", "e2e-tests-crd-tcp_svc_http-echo_http2"}
		for _, beName := range beNames {
			err = suite.checkBackend(p, beName, "mode", &types.StringC{Value: "tcp"})
			suite.Require().NoError(err)
			err = suite.checkBackend(p, beName, "balance", &types.Balance{
				Algorithm: "leastconn",
			})
			suite.Require().NoError(err)
			err = suite.checkBackend(p, beName, "option abortonclose", &types.SimpleOption{NoOption: true, Comment: ""})
			suite.Require().NoError(err)
			err = suite.checkBackend(p, beName, "default-server", []types.DefaultServer{
				{
					Params: []params.ServerOption{
						&params.ServerOptionValue{
							Name:  "check-sni",
							Value: "example.com",
						},
						&params.ServerOptionValue{
							Name:  "resolve-prefer",
							Value: "ipv4",
						},
						&params.ServerOptionValue{
							Name:  "sni",
							Value: "str(example.com)",
						},
						&params.ServerOptionValue{
							Name:  "verify",
							Value: "none",
						},
					},
				},
			})
			suite.Require().NoError(err)
		}
	})
}

func (suite *TCPSuite) Test_CRD_TCP_SSL() {
	suite.Run("TCP CR SSL", func() {
		var err error
		crdPath := e2e.GetCRDFixturePath() + "/tcp-cr-ssl.yaml"
		suite.Require().NoError(suite.test.Apply(crdPath, suite.test.GetNS(), nil))
		suite.client, err = e2e.NewHTTPSClient("crdtcp-test.haproxy", 32766)
		suite.Require().NoError(err)
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
	suite.Require().NoError(err, "Could not get Haproxy config")
	reader := strings.NewReader(cfg)
	p, err := parser.New(options.Reader(reader))
	suite.Require().NoError(err, "Could not get Haproxy config parser")
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
	err = suite.checkFrontend(p, feName443, "bind", binds443)
	suite.Require().NoError(err)
	err = suite.checkFrontend(p, feName443, "mode", &types.StringC{Value: "tcp"})
	suite.Require().NoError(err)
	err = suite.checkFrontend(p, feName443, "log-format", &types.StringC{Value: "'%{+Q}o %t %s'"})
	suite.Require().NoError(err)
	err = suite.checkFrontend(p, feName443, "option tcplog", &types.SimpleOption{NoOption: false, Comment: ""})
	suite.Require().NoError(err)
	err = suite.checkFrontend(p, feName443, "default_backend", &types.StringC{Value: "e2e-tests-crd-tcp_svc_http-echo_https"})
	suite.Require().NoError(err)
}

func (suite *TCPSuite) checkFrontends(p parser.Parser) error {
	// Checks for tcp tcp-http-echo-80
	binds80 := []types.Bind{
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
	feName80 := "tcpcr_e2e-tests-crd-tcp_fe-http-echo-80"
	err := suite.checkFrontend(p, feName80, "bind", binds80)
	if err != nil {
		return err
	}
	err = suite.checkFrontend(p, feName80, "mode", &types.StringC{Value: "tcp"})
	if err != nil {
		return err
	}
	err = suite.checkFrontend(p, feName80, "log-format", &types.StringC{Value: "'%{+Q}o %t %s'"})
	if err != nil {
		return err
	}
	err = suite.checkFrontend(p, feName80, "option tcplog", &types.SimpleOption{NoOption: false, Comment: ""})
	if err != nil {
		return err
	}
	err = suite.checkFrontend(p, feName80, "default_backend", &types.StringC{Value: "e2e-tests-crd-tcp_svc_http-echo_http"})
	if err != nil {
		return err
	}

	// Check for tcp tcp-http-echo-444
	binds81 := []types.Bind{
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
	feName444 := "tcpcr_e2e-tests-crd-tcp_fe-http-echo-81"
	err = suite.checkFrontend(p, feName444, "bind", binds81)
	if err != nil {
		return err
	}
	err = suite.checkFrontend(p, feName444, "mode", &types.StringC{Value: "tcp"})
	if err != nil {
		return err
	}
	err = suite.checkFrontend(p, feName444, "log-format", &types.StringC{Value: "'%{+Q}o %t %s'"})
	if err != nil {
		return err
	}
	err = suite.checkFrontend(p, feName444, "option tcplog", &types.SimpleOption{NoOption: false, Comment: ""})
	if err != nil {
		return err
	}
	err = suite.checkFrontend(p, feName444, "default_backend", &types.StringC{Value: "e2e-tests-crd-tcp_svc_http-echo_http2"})
	if err != nil {
		return err
	}
	return nil
}
