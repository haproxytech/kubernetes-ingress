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

package globalconfig

import (
	"strings"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *GlobalConfigSuite) Test_Proxy_Protocol() {
	suite.Run("Source_IP_OK", func() {
		suite.NoError(suite.test.Apply("config/configmap-pp-1.yaml", "", nil))
		suite.Eventually(func() bool {
			res, err := e2e.ProxyProtoConn()
			if err != nil {
				suite.T().Logf("Connection ERROR: %s", err.Error())
				return false
			}
			return strings.Contains(string(res), "404 Not Found")
		}, e2e.WaitDuration, e2e.TickDuration)
	})

	suite.Run("Source_IP_KO", func() {
		suite.NoError(suite.test.Apply("config/configmap-pp-2.yaml", "", nil))
		suite.Eventually(func() bool {
			res, err := e2e.ProxyProtoConn()
			if err != nil {
				suite.T().Logf("Connection ERROR: %s", err.Error())
				return false
			}
			suite.T().Logf("Result: %s", string(res))
			return strings.Contains(string(res), "400 Bad request")
		}, e2e.WaitDuration, e2e.TickDuration)
	})

	// revert to initial configmap
	suite.NoError(suite.test.Apply("../../config/2.configmap.yaml", "", nil))
}
