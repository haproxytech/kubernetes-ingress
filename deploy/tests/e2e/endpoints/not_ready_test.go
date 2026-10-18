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
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *EndpointsSuite) Test_Non_Ready_Endpoints() {
	suite.tmplData.NotReady = true
	suite.tmplData.Replicas = 3
	suite.Require().NoError(suite.test.Apply("config/endpoints.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	// The single pod created before the template switched to never-ready
	// endpoints is the only one the controller must have kept: with a
	// dual stack service it counts for two servers, one per IP family.
	// The never-ready pods of the new replicas must not be added: check
	// being disabled, they would show up as up servers.
	backendName := suite.test.GetNS() + "_svc_http-echo_http"
	suite.Require().Eventually(func() bool {
		count, err := e2e.GetRuntimeUpServersCount(backendName)
		if err != nil {
			suite.T().Log(err)
			return false
		}
		return count == 2
	}, e2e.WaitDuration, e2e.TickDuration)
	suite.Require().Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		return res.StatusCode == 200
	}, e2e.WaitDuration, e2e.TickDuration)
}
