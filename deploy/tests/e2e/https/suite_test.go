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

// +build e2e_sequential

package https

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type HTTPSSuite struct {
	suite.Suite
	test     e2e.Test
	client   *e2e.Client
	tmplData tmplData
}

type tmplData struct {
	IngAnnotations []struct{ Key, Value string }
	TLSEnabled     bool
	Host           string
	Port           string
}

func (suite *HTTPSSuite) BeforeTest(suiteName, testName string) {
	var err error
	suite.tmplData.IngAnnotations = nil
	suite.tmplData.Port = "http"
	switch testName {
	case "Test_HTTPS_Redirect":
		suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host)
		suite.tmplData.TLSEnabled = false
		suite.client.NoRedirect = true
	default:
		suite.client, err = e2e.NewHTTPSClient(suite.tmplData.Host)
		suite.tmplData.TLSEnabled = true
	}
	suite.NoError(err)
}

func (suite *HTTPSSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.NoError(err)
	suite.tmplData = tmplData{Host: suite.test.GetNS() + ".test"}
	suite.NoError(suite.test.DeployYaml("config/deploy.yaml", suite.test.GetNS()))
}

func (suite *HTTPSSuite) TearDownSuite() {
	suite.test.TearDown()
}

func TestHTTPSSuite(t *testing.T) {
	suite.Run(t, new(HTTPSSuite))
}
