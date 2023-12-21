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

package adminport

import (
	"net/http"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *AdminPortSuite) Test_Pprof() {
	suite.Run("OK", func() {
		suite.Eventually(func() bool {
			suite.client.Path = "/debug/pprof/cmdline"
			res, cls, err := suite.client.Do()
			if err != nil {
				suite.T().Logf("Connection ERROR: %s", err.Error())
				return false
			}
			defer cls()
			return res.StatusCode == http.StatusOK
		}, e2e.WaitDuration, e2e.TickDuration)
	})
	suite.Run("OK", func() {
		suite.Eventually(func() bool {
			suite.client.Path = "/debug/pprof/symbol"
			res, cls, err := suite.client.Do()
			if err != nil {
				suite.T().Logf("Connection ERROR: %s", err.Error())
				return false
			}
			defer cls()
			return res.StatusCode == http.StatusOK
		}, e2e.WaitDuration, e2e.TickDuration)
	})
}
