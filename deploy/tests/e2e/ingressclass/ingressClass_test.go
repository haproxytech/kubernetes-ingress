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

package ingressclass

import (
	"net/http"
	"os/exec"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type echoServerResponse struct {
	OS struct {
		Hostname string `json:"hostname"`
	} `json:"os"`
}

func (suite *IngressClassSuite) Test_IngressClassName_Field() {
	test := suite.test
	suite.Run("Disabled", func() {
		suite.NoError(test.DeployYamlTemplate("config/ingress.yaml.tmpl", test.GetNS(), suite.tmplData))
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if err != nil {
				return false
			}
			defer cls()

			return res.StatusCode == http.StatusServiceUnavailable || res.StatusCode == http.StatusNotFound
		}, e2e.WaitDuration, e2e.TickDuration)
	})

	suite.Run("Enabled", func() {
		suite.tmplData.IngressClassName = "haproxy"
		suite.NoError(test.DeployYamlTemplate("config/ingress.yaml.tmpl", test.GetNS(), suite.tmplData))
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if err != nil {
				return false
			}
			defer cls()

			return res.StatusCode == http.StatusOK
		}, e2e.WaitDuration, e2e.TickDuration)
	})
}

func (suite *IngressClassSuite) Test_IngressClassName_Resource() {
	test := suite.test
	suite.Run("Disabled", func() {
		cmd := exec.Command("kubectl", "delete", "ingressclasses", "haproxy")
		suite.NoError(cmd.Run())

		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if err != nil {
				return false
			}
			defer cls()

			return res.StatusCode == http.StatusServiceUnavailable || res.StatusCode == http.StatusNotFound
		}, e2e.WaitDuration, e2e.TickDuration)
	})

	suite.Run("Enabled", func() {
		suite.NoError(test.DeployYaml("config/deploy.yaml", test.GetNS()))
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if err != nil {
				return false
			}
			defer cls()

			return res.StatusCode == http.StatusOK
		}, e2e.WaitDuration, e2e.TickDuration)
	})
}
