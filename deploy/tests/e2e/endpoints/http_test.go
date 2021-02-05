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

package endpoints

import (
	"fmt"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *EndpointsSuite) Test_HTTP_Reach() {
	for _, replicas := range []int{2, 4} {
		suite.Run(fmt.Sprintf("%d-replicas", replicas), func() {
			suite.tmplData.Replicas = replicas
			suite.test.DeployYamlTemplate("config/endpoints.yaml.tmpl", suite.test.GetNS(), suite.tmplData)
			suite.Eventually(func() bool {
				counter := map[string]int{}
				for i := 0; i < replicas*2; i++ {
					func() {
						res, cls, err := suite.client.Do()
						suite.NoError(err)
						defer cls()
						counter[newReachResponse(suite.T(), res)]++
					}()
				}
				for _, v := range counter {
					if v != 2 {
						return false
					}
				}
				return true
			}, e2e.WaitDuration, e2e.TickDuration)
		})
	}
}
