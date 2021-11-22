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

//go:build e2e_sequential

package endpoints

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

// For each test we send two times mores requests than replicas available.
// In the end the counter should be 2 for each returned pod-name in request answer.
func (suite *EndpointsSuite) Test_HTTP_Reach() {
	pid := suite.getPID()
	for _, replicas := range []int{4, 8, 2, 0, 3} {
		suite.Run(fmt.Sprintf("%d-replicas", replicas), func() {
			suite.tmplData.Replicas = replicas
			suite.NoError(suite.test.Apply("config/endpoints.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
			suite.Eventually(func() bool {
				counter := map[string]int{}
				for i := 0; i < replicas*2; i++ {
					func() {
						res, cls, err := suite.client.Do()
						suite.NoError(err)
						defer cls()
						if res.StatusCode == http.StatusOK {
							body, err := ioutil.ReadAll(res.Body)
							if err != nil {
								suite.Error(err)
								return
							}
							counter[string(body)]++
						}
					}()
				}
				switch replicas {
				case 8, 3:
					// HAProxy reloaded due to scale up
					pid = suite.getPID()
				case 0:
					if len(counter) > 0 {
						return false
					}
				default:
					if pid != suite.getPID() {
						suite.Error(fmt.Errorf("Uncessary reload of HAproxy"))
						return false
					}
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

func (suite *EndpointsSuite) getPID() (pid string) {
	r, err := e2e.GetGlobalHAProxyInfo()
	if err != nil {
		suite.T().Log(err)
		return
	}
	pid = r.Pid
	return
}
