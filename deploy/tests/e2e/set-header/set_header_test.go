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

package setheader

import (
	"encoding/json"
	"io/ioutil"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type test struct {
	headerName  string
	headerValue string
}

func (suite *SetHeaderSuite) Test_Request_Set_Header() {
	for _, tc := range []test{
		{"Cache-Control", "no-store,no-cache,private"},
		{"X-Custom-Header", "haproxy-ingress-controller"},
	} {
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{"request-set-header", tc.headerName + " " + tc.headerValue},
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
			type echo struct {
				HTTP struct {
					Headers map[string]string `json:"headers"`
				} `json:"http"`
			}
			e := &echo{}
			if err := json.Unmarshal(b, e); err != nil {
				return false
			}
			v, ok := e.HTTP.Headers[tc.headerName]
			if !ok {
				return false
			}
			return v == tc.headerValue
		}, e2e.WaitDuration, e2e.TickDuration)
	}
}

func (suite *SetHeaderSuite) Test_Response_Set_Header() {
	for _, tc := range []test{
		{"Cache-Control", "no-store,no-cache,private"},
		{"X-Custom-Header", "haproxy-ingress-controller"},
	} {
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{"response-set-header", tc.headerName + " " + tc.headerValue},
		}
		suite.Require().NoError(suite.test.DeployYamlTemplate("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Eventually(func() bool {
			r, cls, err := suite.client.Do()
			if err != nil {
				return false
			}
			defer cls()
			return r.Header.Get(tc.headerName) == tc.headerValue
		}, e2e.WaitDuration, e2e.TickDuration)
	}
}
