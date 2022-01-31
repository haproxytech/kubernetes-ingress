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

package accesscontrol

import (
	"net/http"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *AccessControlSuite) eventuallyReturns(clientIP string, httpStatus int) {
	suite.Eventually(func() bool {
		suite.client.Req.Header = map[string][]string{
			"X-Client-IP": {clientIP},
		}
		res, cls, err := suite.client.Do()
		if err != nil {
			suite.T().Logf("Connection ERROR: %s", err.Error())
			return false
		}
		defer cls()
		return res.StatusCode == httpStatus
	}, e2e.WaitDuration, e2e.TickDuration, "waiting for call with client IP %v to return %v expired", clientIP, httpStatus)
}

func (suite *AccessControlSuite) Test_Whitelist() {
	suite.Run("Inline", func() {
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{"src-ip-header", " X-Client-IP"},
			{"whitelist", " 192.168.2.0/24"},
		}

		suite.NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))

		suite.eventuallyReturns("192.168.2.3", http.StatusOK)
		suite.eventuallyReturns("192.168.5.3", http.StatusForbidden)
	})

	suite.Run("Patternfile", func() {
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{"src-ip-header", " X-Client-IP"},
			{"whitelist", " patterns/ips"},
		}

		suite.NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.NoError(suite.test.Apply("config/patternfile-a.yml", "", nil))

		suite.eventuallyReturns("192.168.0.3", http.StatusOK)
		suite.eventuallyReturns("192.168.2.3", http.StatusForbidden)
	})
}

func (suite *AccessControlSuite) Test_Blacklist() {
	suite.Run("Inline", func() {
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{"src-ip-header", " X-Client-IP"},
			{"blacklist", " 192.168.2.0/24"},
		}

		suite.NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))

		suite.eventuallyReturns("192.168.2.3", http.StatusForbidden)
		suite.eventuallyReturns("192.168.5.3", http.StatusOK)
	})

	suite.Run("Patternfile", func() {
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{"src-ip-header", " X-Client-IP"},
			{"blacklist", " patterns/ips"},
		}

		suite.NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.NoError(suite.test.Apply("config/patternfile-a.yml", "", nil))

		suite.eventuallyReturns("192.168.0.3", http.StatusForbidden)
		suite.eventuallyReturns("192.168.2.3", http.StatusOK)
	})
}
