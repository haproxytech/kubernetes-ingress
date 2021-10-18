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

package https

import (
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *HTTPSSuite) Test_HTTPS_Offload() {
	var err error
	suite.NoError(suite.test.DeployYamlTemplate("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.client, err = e2e.NewHTTPSClient("offload-test.haproxy", 0)
	suite.NoError(err)
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		targetCrt := res.TLS.PeerCertificates[0].Subject.CommonName == "offload-test.haproxy"
		return targetCrt
	}, e2e.WaitDuration, e2e.TickDuration)
}
