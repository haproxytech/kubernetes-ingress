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

package mapupdate

import (
	"strconv"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *MapUpdateSuite) Test_Update() {
	suite.Run("Update", func() {
		suite.tmplData.Paths = make([]string, 0, 700)
		for i := 0; i < 700; i++ {
			suite.tmplData.Paths = append(suite.tmplData.Paths, strconv.Itoa(i))
		}
		oldInfo, err := e2e.GetGlobalHAProxyInfo()
		suite.NoError(err)
		suite.NoError(suite.test.Apply("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Require().Eventually(func() bool {
			newInfo, err := e2e.GetGlobalHAProxyInfo()
			suite.NoError(err)
			count, err := e2e.GetHAProxyMapCount("path-prefix")
			suite.NoError(err)
			suite.T().Log(count)
			return oldInfo.Pid == newInfo.Pid && count == 701 // 700 + default_http-echo_http
		}, e2e.WaitDuration, e2e.TickDuration)
	})
}
