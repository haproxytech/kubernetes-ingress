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

package scale

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type ScaleServerSuite struct {
	suite.Suite
	test     e2e.Test
	client   *e2e.Client
	tmplData tmplData
}

type tmplData struct {
	Host                string
	Replicas            int
	BackendCR           bool
	BackendCRWithReload bool
	Namespace           string
}

func (suite *ScaleServerSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.Require().NoError(err)
	suite.tmplData = tmplData{Host: suite.test.GetNS() + ".test", Namespace: suite.test.GetNS()}
}

func (suite *ScaleServerSuite) TearDownSuite() {
	suite.test.TearDown()
}

func (suite *ScaleServerSuite) TearDownTest() {
	suite.Require().NoError(suite.test.DeleteInNamespace("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
}

func TestScaleServerSuite(t *testing.T) {
	suite.Run(t, new(ScaleServerSuite))
}

// Wait for the right number of servers to be up on the runtime
func (suite *ScaleServerSuite) WaitForUpServersOnRuntime(backend string, count int) {
	upServersCount := -1
	var err error
	suite.Require().Eventually(func() bool {
		upServersCount, err = e2e.GetRuntimeUpServersCount(backend)
		if err != nil {
			suite.T().Logf("ERROR: %s", err.Error())
			return false
		}
		return upServersCount == count
	}, e2e.WaitDuration, e2e.TickDuration, fmt.Sprintf("upServerCount(%d) != %d", upServersCount, count))
	return
}
