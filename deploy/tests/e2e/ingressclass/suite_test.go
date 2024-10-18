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

package ingressclass

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type IngressClassSuite struct {
	suite.Suite
	test     e2e.Test
	client   *e2e.Client
	tmplData tmplData
}

type tmplData struct {
	IngressClassName string
	Host             string
}

func (suite *IngressClassSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.Require().NoError(err)
	major, minor, err := suite.test.GetK8sVersion()
	suite.Require().NoError(err)
	if major == 1 && minor < 18 {
		suite.T().SkipNow()
	}
	suite.tmplData = tmplData{Host: suite.test.GetNS() + ".test"}
	suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml", suite.test.GetNS(), nil))
	suite.Require().NoError(suite.test.Apply("config/ingressclass.yaml", "", nil))
	suite.test.AddTearDown(func() error {
		return suite.test.Delete("config/ingressclass.yaml")
	})
}

func (suite *IngressClassSuite) TearDownSuite() {
	err := suite.test.TearDown()
	if err != nil {
		suite.T().Error(err)
	}
}

func TestIngressClassSuite(t *testing.T) {
	suite.Run(t, new(IngressClassSuite))
}
