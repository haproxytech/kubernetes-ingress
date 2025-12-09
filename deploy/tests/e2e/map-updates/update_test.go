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
		n := 700
		suite.tmplData.Paths = make([]string, 0, n)
		for i := 0; i < n; i++ {
			suite.tmplData.Paths = append(suite.tmplData.Paths, strconv.Itoa(i))
		}
		oldInfo, err := e2e.GetGlobalHAProxyInfo()
		oldCountExact, err := e2e.GetHAProxyMapCount("path-exact")
		suite.Require().NoError(err)
		oldCountPrefixExact, err := e2e.GetHAProxyMapCount("path-prefix-exact")
		suite.Require().NoError(err)
		oldCountPrefix, err := e2e.GetHAProxyMapCount("path-prefix")
		suite.Require().NoError(err)
		suite.Require().NoError(suite.test.Apply("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Require().Eventually(func() bool {
			newInfo, err := e2e.GetGlobalHAProxyInfo()
			if err != nil {
				suite.T().Log(err)
				return false
			}
			countExact, err := e2e.GetHAProxyMapCount("path-exact")
			if err != nil {
				suite.T().Log(err)
				return false
			}
			countPrefixExact, err := e2e.GetHAProxyMapCount("path-prefix-exact")
			if err != nil {
				suite.T().Log(err)
				return false
			}
			countPrefix, err := e2e.GetHAProxyMapCount("path-prefix")
			if err != nil {
				suite.T().Log(err)
				return false
			}
			numOfAddedEntriesExact := countExact - oldCountExact
			numOfAddedEntriesPrefixExact := countPrefixExact - oldCountPrefixExact
			numOfAddedEntriesPrefix := countPrefix - oldCountPrefix + 1 // We add one because there's already an entry at the beginning which will be removed
			suite.T().Logf("oldInfo.Pid(%s) == newInfo.Pid(%s) && additional path-exact.count(%d) == %d && additional path-prefix-exact.count(%d) == %d && additional path-prefix.count(%d) == %d", oldInfo.Pid, newInfo.Pid, numOfAddedEntriesExact, 0, numOfAddedEntriesPrefixExact, n, numOfAddedEntriesPrefix, n)
			return oldInfo.Pid == newInfo.Pid && numOfAddedEntriesExact == 0 && numOfAddedEntriesPrefixExact == n && numOfAddedEntriesPrefix == n
		}, e2e.WaitDuration, e2e.TickDuration)
	})
}
