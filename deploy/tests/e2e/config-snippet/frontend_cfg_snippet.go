// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed uner the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build e2e_parallel

package configsnippet

import (
	"encoding/json"
	"io/ioutil"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *ConfigSnippetSuite) TestFrontendCfgSnippet() {
	suite.NoError(suite.test.Apply("config/configmap.yaml", "", nil))
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
		_, ok := e.HTTP.Headers["X-Unique-Id"]
		return ok
	}, e2e.WaitDuration, e2e.TickDuration)
}
