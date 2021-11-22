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

//go:build e2e_parallel

package canarydeployment

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type CanaryDeploymentSuite struct {
	suite.Suite
	test     e2e.Test
	client   *e2e.Client
	tmplData tmplData
}

type tmplData struct {
	Host            string
	StagingRouteACL string
}

func (suite *CanaryDeploymentSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.NoError(err)
	suite.tmplData = tmplData{
		Host:            suite.test.GetNS() + ".test",
		StagingRouteACL: "rand(100) lt 0",
	}
	suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host)
	suite.NoError(err)
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Require().Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		if res.StatusCode == 200 {
			body, _ := ioutil.ReadAll(res.Body)
			return strings.HasPrefix(string(body), "http-echo-prod")
		}
		return false
	}, e2e.WaitDuration, e2e.TickDuration)
}

func (suite *CanaryDeploymentSuite) TearDownSuite() {
	suite.test.TearDown()
}

func TestCanaryDeploymentSuite(t *testing.T) {
	suite.Run(t, new(CanaryDeploymentSuite))
}
