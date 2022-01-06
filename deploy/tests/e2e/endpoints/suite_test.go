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

package endpoints

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type EndpointsSuite struct {
	suite.Suite
	test     e2e.Test
	client   *e2e.Client
	tmplData tmplData
}

type tmplData struct {
	Replicas int
	Host     string
	NotReady bool
}

func (suite *EndpointsSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.NoError(err)
	suite.tmplData = tmplData{
		Host: suite.test.GetNS() + ".test",
	}
}

func (suite *EndpointsSuite) TearDownSuite() {
	err := suite.test.TearDown()
	if err != nil {
		suite.T().Error(err)
	}
}

func TestEndpointsSuite(t *testing.T) {
	suite.Run(t, new(EndpointsSuite))
}

func (suite *EndpointsSuite) BeforeTest(suiteName, testName string) {
	var err error
	test := suite.test
	switch testName {
	case "Test_TCP_Reach":
		suite.client, err = e2e.NewHTTPSClient("tcp-service.test", 32766)
		suite.NoError(err)
		suite.tmplData.Replicas = 4
		suite.NoError(test.Apply("config/endpoints.yaml.tmpl", test.GetNS(), suite.tmplData))
		suite.NoError(test.Apply("config/tcp.yaml", "", nil))
		test.AddTearDown(func() error {
			return suite.test.Delete("config/tcp.yaml")
		})
		suite.Require().Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			return res.StatusCode == http.StatusOK
		}, e2e.WaitDuration, e2e.TickDuration)
	default:
		suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host)
		suite.NoError(err)
		suite.tmplData.Replicas = 1
		suite.NoError(test.Apply("config/endpoints.yaml.tmpl", test.GetNS(), suite.tmplData))
		suite.Require().Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			return res.StatusCode == http.StatusOK
		}, e2e.WaitDuration, e2e.TickDuration)
	}
}
