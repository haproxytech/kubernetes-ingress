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

package canarydeployment

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *CanaryDeploymentSuite) Test_Response_Percentage() {
	for _, percentage := range []int{0, 25, 100} {
		suite.Run(fmt.Sprintf("%d", percentage), func() {
			suite.tmplData.StagingRouteACL = fmt.Sprintf("rand(100) lt %d", percentage)
			suite.NoError(suite.test.DeployYamlTemplate("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
			suite.Eventually(func() bool {
				counter := 0
				for i := 0; i < 10; i++ {
					res, cls, err := suite.client.Do()
					suite.NoError(err)
					defer cls()
					if res.StatusCode == 200 {
						body, _ := ioutil.ReadAll(res.Body)
						if strings.HasPrefix(string(body), "http-echo-staging") {
							counter++
						}
					}
				}
				switch percentage {
				case 0:
					if counter == 0 {
						return true
					}
				case 100:
					if counter == 10 {
						return true
					}
				case 25:
					{
						if counter > 2 && counter < 5 {
							return true
						}
					}
				}
				suite.T().Logf("counter:%d", counter)
				return false
			}, e2e.WaitDuration, e2e.TickDuration)
		})
	}
}
