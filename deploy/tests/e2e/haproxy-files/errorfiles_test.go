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

package haproxyfiles

import (
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *HAProxyFilesSuite) Test_ErrorFiles() {
	suite.Run("Enabled", func() {
		suite.NoError(suite.test.Apply("config/errorfiles.yaml", "", nil))
		suite.Require().Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			return res.StatusCode == 521
		}, e2e.WaitDuration, e2e.TickDuration)
	})

	suite.Run("Disabled", func() {
		suite.NoError(suite.test.Delete("config/errorfiles.yaml"))
		suite.Require().Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			return res.StatusCode == 503
		}, e2e.WaitDuration, e2e.TickDuration)
	})
}
