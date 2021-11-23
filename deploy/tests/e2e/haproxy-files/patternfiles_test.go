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
	"strconv"
)

var tests1 = []string{
	"H+DbITVEMAJDRS4EzVF4gVDfTmyUFB3RzEkCIST9",
	"GBxqv9YZHQ5dBLMjTjBbyjd4/wS0F0ETZtsnLsYI",
	"7tFRKRoboeFENmfTSHj+gjJKFjtOw2u+G+1d13rO",
}

var tests2 = []string{
	"F3ITB+V5w0Yo8ZeS8LO3mkNlFBOxS3i9TDYfVjJv",
	"Yn9MvTzjAeLqp7MKE7g7thlf6jc2WRYTNoya+Cqb",
	"Y6FX16EEqxJr/B9M2Pzzt/NvivoDjZE2FTr4boBb",
}

func (suite *HAProxyFilesSuite) Test_PatternFiles() {
	suite.Run("Enabled", func() {
		for i, path := range tests1 {
			suite.Require().Eventually(func() bool {
				return suite.testHeader(path, strconv.Itoa(i))
			}, e2e.WaitDuration, e2e.TickDuration)
		}
	})
	suite.Run("Updated", func() {
		suite.NoError(suite.test.Apply("config/patternfiles-2.yaml", "", nil))
		for i, path := range tests2 {
			suite.Require().Eventually(func() bool {
				return suite.testHeader(path, strconv.Itoa(i))
			}, e2e.WaitDuration, e2e.TickDuration)
		}
	})
}

func (suite *HAProxyFilesSuite) testHeader(in, out string) bool {
	suite.client.Path = "/" + in
	res, cls, err := suite.client.Do()
	if res == nil {
		suite.T().Log(err)
		return false
	}
	defer cls()
	v, ok := res.Header["Result"]
	if !ok {
		suite.T().Logf("result header not found in %s", res.Header)
		return false
	}
	if v[0] != out {
		return false
	}
	return true
}
