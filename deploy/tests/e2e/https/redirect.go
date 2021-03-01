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
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *HTTPSSuite) Test_HTTPS_Redirect() {
	suite.NoError(suite.test.DeployYamlTemplate("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Require().Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		return res.StatusCode == 200
	}, e2e.WaitDuration, e2e.TickDuration)
	suite.Run("implicit_via_TLS", func() {
		suite.tmplData.TLSEnabled = true
		suite.Eventually(func() bool {
			suite.NoError(suite.test.DeployYamlTemplate("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			return res.StatusCode == 302
		}, e2e.WaitDuration, e2e.TickDuration)
	})
	suite.Run("custom_code", func() {
		suite.tmplData.TLSEnabled = false
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{"ssl-redirect", "'true'"},
			{"ssl-redirect-code", "'301'"},
		}
		suite.NoError(suite.test.DeployYamlTemplate("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			return res.StatusCode == 301
		}, e2e.WaitDuration, e2e.TickDuration)
	})
	suite.Run("disabled", func() {
		suite.tmplData.TLSEnabled = true
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{"ssl-redirect", "'false'"},
		}
		suite.NoError(suite.test.DeployYamlTemplate("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			return res.StatusCode == 200
		}, e2e.WaitDuration, e2e.TickDuration)
	})
}
