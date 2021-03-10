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

// +build e2e_parallel

package servicediscovery

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type ServiceDiscoverySuite struct {
	suite.Suite
	test     e2e.Test
	client   *e2e.Client
	tmplData tmplData
}

type tmplData struct {
	Host        string
	ServiceName string
	ServicePort string
}

func (suite *ServiceDiscoverySuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.NoError(err)
	suite.tmplData = tmplData{
		Host:        suite.test.GetNS() + ".test",
		ServiceName: "http-echo-1",
		ServicePort: "8080",
	}
	suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host)
	suite.NoError(err)
	suite.NoError(suite.test.DeployYaml("config/deploy.yaml", suite.test.GetNS()))
	suite.NoError(suite.test.DeployYamlTemplate("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
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

func (suite *ServiceDiscoverySuite) TearDownSuite() {
	suite.test.TearDown()
}

func TestServiceDiscoverySuite(t *testing.T) {
	suite.Run(t, new(ServiceDiscoverySuite))
}
