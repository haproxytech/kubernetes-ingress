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

package crdfrontend

import (
	"testing"

	models "github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

// Adding FrontendSuite, just to be able to debug directly here and not from CRDSuite
type FrontendSuite struct {
	CRDFrontendSuite
}

func TestFrontendSuite(t *testing.T) {
	suite.Run(t, new(FrontendSuite))
}

func (suite *FrontendSuite) Test_CR_Frontend() {
	var port int64 = 8080
	var portTest int64 = 9080
	binds := map[string]models.Bind{
		"v4": {
			BindParams: models.BindParams{
				Name: "v4",
			},
			Address: "0.0.0.0",
			Port:    &port,
		},
		"v6": {
			BindParams: models.BindParams{
				Name: "v6",
			},
			Address: "::",
			Port:    &port,
		},
		"test-http": {
			BindParams: models.BindParams{
				Name: "test-http",
			},
			Address: "127.0.0.1",
			Port:    &portTest,
		},
	}

	suite.Run("CRs OK", func() {
		// Add HTTP frontend custom resource
		frontendPath := "config/frontend-http.yaml"
		suite.Require().NoError(suite.test.Apply(frontendPath, "", nil))
		suite.test.AddTearDown(func() error {
			return suite.test.Delete(frontendPath)
		})

		// Add frontend custom resource to configmap
		configmapFrontendPath := "config/configmap-frontend-http.yaml"
		suite.Require().NoError(suite.test.Apply(configmapFrontendPath, "", nil))
		suite.test.AddTearDown(func() error {
			return suite.test.Apply("../../config/2.configmap.yaml", "", nil)
		})

		suite.Require().Eventually(func() bool {
			httpFrontend, err := suite.getFrontendConfiguration("http")
			if err != nil || httpFrontend == nil {
				return false
			}

			return httpFrontend.Name == "http" && httpFrontend.Mode == "http" &&
				EqualBinds(httpFrontend.Binds, binds)
		}, e2e.WaitDuration, e2e.TickDuration, "waiting for HTTP frontend custom resource to be applied")
	})
}

func EqualBinds(left, right map[string]models.Bind) bool {
	if len(left) != len(right) {
		return false
	}
	for k, bind := range left {
		if !bind.Equal(right[k]) {
			return false
		}
	}
	return true
}
