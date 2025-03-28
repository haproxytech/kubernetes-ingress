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

package ingressmatch

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type IngressMatchSuite struct {
	suite.Suite
	test     e2e.Test
	client   *e2e.Client
	tmplData tmplData
}

type IngressRule struct {
	Host     string
	Path     string
	PathType string
	Service  string
}

type tmplData struct {
	PathTypeSupported bool
	Apps              []int
	Rules             []IngressRule
}

func (suite *IngressMatchSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.Require().NoError(err)
	suite.client, err = e2e.NewHTTPClient(suite.test.GetNS() + ".test")
	suite.Require().NoError(err)
}

func (suite *IngressMatchSuite) TearDownSuite() {
	suite.test.TearDown()
}

func TestIngressMatchSuite(t *testing.T) {
	suite.Run(t, new(IngressMatchSuite))
}
