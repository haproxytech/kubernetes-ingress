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

package sourceip

import (
	"net/http"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *SourceIPSuite) Test_Set_Source_Ip() {
	type tc struct {
		HeaderName string
		IpValue    string
	}
	for _, tc := range []tc{
		{"X-Client-IP", "10.0.0.1"},
		{"X-Real-IP", "62.1.87.32"},
	} {
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{"src-ip-header", tc.HeaderName},
			{"blacklist", tc.IpValue},
		}
		suite.Run(tc.HeaderName, func() {
			suite.NoError(suite.test.DeployYamlTemplate("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
			suite.Eventually(func() bool {
				suite.client.Req.Header = map[string][]string{
					tc.HeaderName: {tc.IpValue},
				}
				res, cls, err := suite.client.Do()
				if err != nil {
					suite.FailNow(err.Error())
				}
				defer cls()
				return res.StatusCode != http.StatusOK
			}, e2e.WaitDuration, e2e.TickDuration)
		})
	}
}
