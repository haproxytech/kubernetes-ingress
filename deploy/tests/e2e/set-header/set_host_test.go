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

// +build integration

package setheader

import (
	"encoding/json"
	"io/ioutil"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *SetHeaderSuite) Test_Set_Host() {
	for _, host := range []string{"foo", "bar"} {
		suite.Run(host, func() {
			suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
				{"set-host", host},
			}
			suite.Require().NoError(suite.test.DeployYamlTemplate("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
			suite.Eventually(func() bool {
				res, cls, err := suite.client.Do()
				if err != nil {
					suite.FailNow(err.Error())
				}
				defer cls()
				b, err := ioutil.ReadAll(res.Body)
				if err != nil {
					return false
				}
				type echoServerResponse struct {
					HTTP struct {
						Host string `json:"host"`
					} `json:"http"`
				}
				e := &echoServerResponse{}
				if err := json.Unmarshal(b, e); err != nil {
					return false
				}
				return e.HTTP.Host == host
			}, e2e.WaitDuration, e2e.TickDuration)
		})
	}
}
