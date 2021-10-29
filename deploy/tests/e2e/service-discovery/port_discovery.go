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

package servicediscovery

import (
	"io/ioutil"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *ServiceDiscoverySuite) Test_Port_Discovery() {
	suite.Run("port_number", func() {
		suite.testServicePort("http-echo-2", "8080")
	})
	suite.Run("port_name", func() {
		suite.testServicePort("http-echo-1", "http")
	})
}

func (suite *ServiceDiscoverySuite) testServicePort(serviceName, servicePort string) {
	suite.tmplData.ServiceName = serviceName
	suite.tmplData.ServicePort = servicePort
	suite.NoError(suite.test.DeployYamlTemplate("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return false
		}
		return strings.HasPrefix(string(body), serviceName)
	}, e2e.WaitDuration, e2e.TickDuration)

}
