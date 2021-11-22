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

package https

import (
	"encoding/json"
	"io/ioutil"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *HTTPSSuite) Test_HTTPS_Passthrough() {
	suite.tmplData.Port = "https"
	suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
		{"ssl-passthrough", "'true'"},
	}
	suite.Require().NoError(suite.test.Apply("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Run("Reach_Backend", func() {
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				return false
			}
			type echoServerResponse struct {
				TLS struct {
					SNI string `json:"sni"`
				} `json:"tls"`
			}
			response := &echoServerResponse{}
			err = json.Unmarshal(body, response)
			if err != nil {
				return false
			}
			return response.TLS.SNI == suite.tmplData.Host
		}, e2e.WaitDuration, e2e.TickDuration)
	})
	suite.Run("Ingress_annotations", func() {
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{"ssl-passthrough", "'true'"},
			{"whitelist", "6.6.6.6"},
		}
		suite.NoError(suite.test.Apply("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if err == nil {
				defer cls()
			}
			// should be blocked (TCP reject) by whitelist since sni matches
			return res == nil
		}, e2e.WaitDuration, e2e.TickDuration)
		suite.client.Host += ".bar"
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if err == nil {
				defer cls()
			}
			// should not be blocked by whitelist rule since sni does not match
			return res != nil
		}, e2e.WaitDuration, e2e.TickDuration)
	})
}
