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

// +build e2e_sequential

package globalconfig

import (
	"os/exec"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *GlobalConfigSuite) TestMaxconn() {
	cmd := exec.Command("kubectl", "apply", "-f", "config/configmap.yaml")
	_, err := cmd.CombinedOutput()
	suite.Require().NoError(err)
	suite.maxconn = "1111"
	suite.Eventually(suite.checkMaxconn, e2e.WaitDuration, e2e.TickDuration)

	cmd = exec.Command("kubectl", "apply", "-f", "../../config/3.configmap.yaml")
	_, err = cmd.CombinedOutput()
	suite.Require().NoError(err)
	suite.maxconn = "1000"
	suite.Eventually(suite.checkMaxconn, e2e.WaitDuration, e2e.TickDuration)
}

func (suite *GlobalConfigSuite) checkMaxconn() bool {
	r, err := e2e.GetGlobalHAProxyInfo()
	if err != nil {
		suite.T().Log(err)
		return false
	}
	return r.Maxconn == suite.maxconn
}
