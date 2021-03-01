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

package basicauth

import (
	"net/http"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *HTTPBasicAuthSuite) Test_BasicAuth() {
	suite.Run("Denied", func() {
		suite.NoError(suite.test.DeployYamlTemplate("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Require().Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			return res.StatusCode == http.StatusUnauthorized
		}, e2e.WaitDuration, e2e.TickDuration)
	})
	suite.Run("Allowed", func() {
		for _, user := range []string{"des", "md5", "sha-256", "sha-512"} {
			suite.Eventually(func() bool {
				suite.client.Req.SetBasicAuth(user, "password")
				res, cls, err := suite.client.Do()
				if err != nil {
					suite.FailNow(err.Error())
				}
				defer cls()
				return res.StatusCode == http.StatusOK
			}, e2e.WaitDuration, e2e.TickDuration)
		}
	})
}
