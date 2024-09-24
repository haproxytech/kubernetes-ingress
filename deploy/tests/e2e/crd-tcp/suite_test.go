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

//go:build e2e_parallel || e2e_sequential

package crdtcp

import (
	"fmt"
	"io"
	"strings"

	"github.com/stretchr/testify/suite"

	parser "github.com/haproxytech/config-parser/v5"
	"github.com/haproxytech/config-parser/v5/common"
	"github.com/haproxytech/config-parser/v5/params"
	"github.com/haproxytech/config-parser/v5/types"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type CRDTCPSuite struct {
	suite.Suite
	test     e2e.Test
	client   *e2e.Client
	tmplData tmplData
}

type tmplData struct {
	Host               string
	Host2              string
	BackendCrNamespace string
	BackendCrName      string
	EchoAppIndex       int
}

func (suite *CRDTCPSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.NoError(err)
	suite.tmplData = tmplData{
		Host:  suite.test.GetNS() + ".test",
		Host2: suite.test.GetNS() + ".test2",
	}
	suite.tmplData.BackendCrNamespace = suite.test.GetNS()
	suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host)
	suite.NoError(err)
	suite.NoError(suite.test.Apply("config/tcp-secret.yaml", suite.test.GetNS(), nil))
	suite.NoError(suite.test.Apply("config/backend-cr.yaml", suite.test.GetNS(), nil))
	nbEchoApps := 3
	for i := 0; i < nbEchoApps; i++ {
		suite.tmplData.EchoAppIndex = i
		suite.NoError(suite.test.Apply("config/deploy-index.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	}
}

func (suite *CRDTCPSuite) TearDownSuite() {
	suite.test.TearDown()
}

func (suite *CRDTCPSuite) BeforeTest(suiteName, testName string) {
	suite.tmplData.BackendCrName = ""
	switch testName {
	case "Test_CRD_TCP_CR_Backend":
		suite.tmplData.BackendCrName = "mybackend"
	}
	suite.NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
}

func (suite *CRDTCPSuite) checkFrontend(p parser.Parser, frontendName, param string, value common.ParserData) {
	v, err := p.Get(parser.Frontends, frontendName, param)
	suite.NoError(err, "Could not get Haproxy config parser Frontend %s param %s", frontendName, param)
	suite.Equal(value, v, fmt.Sprintf("Frontend param %s should be equal to %v but is %v", param, value, v))
}

func (suite *CRDTCPSuite) checkBackend(p parser.Parser, backendName, param string, value common.ParserData) {
	v, err := p.Get(parser.Backends, backendName, param)
	suite.NoError(err, "Could not get Haproxy config parser Frontend %s param %s", backendName, param)
	suite.Equal(value, v, fmt.Sprintf("Backend param %s should be equal to %v but is %v", param, value, v))
}

func (suite *CRDTCPSuite) checkClientRequest(host, backend string) {
	var err error
	suite.client, err = e2e.NewHTTPSClient(host, 32766)
	suite.NoError(err)
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
			ok = strings.HasPrefix(string(body), backend)
		}
		return ok
	}, e2e.WaitDuration, e2e.TickDuration)
}

func (suite *CRDTCPSuite) checkBasicHttpEchoFrontend(p parser.Parser, feName string) {
	// Checks for tcpcr_e2e-tests-crd-tcp_fe-http-echo
	binds443 := []types.Bind{
		{
			Path: "0.0.0.0:32766",
			Params: []params.BindOption{
				&params.BindOptionValue{
					Name:  "name",
					Value: "v4",
				},
			},
		},
	}
	suite.checkFrontend(p, feName, "bind", binds443)
	suite.checkFrontend(p, feName, "mode", &types.StringC{Value: "tcp"})
	suite.checkFrontend(p, feName, "log-format", &types.StringC{Value: "'%{+Q}o %t %s'"})
	suite.checkFrontend(p, feName, "option tcplog", &types.SimpleOption{NoOption: false, Comment: ""})
	suite.checkFrontend(p, feName, "default_backend", &types.StringC{Value: "e2e-tests-crd-tcp_http-echo_https"})
}
