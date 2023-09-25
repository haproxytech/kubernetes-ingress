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

package tlsauth

import (
	"crypto/tls"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *TLSAuthSuite) Test_Client_TLS_Auth() {
	suite.Run("no_client_cert", func() {
		suite.Require().Eventually(func() bool {
			_, cls, err := suite.client.Do()
			if err != nil {
				suite.T().Log(err)
				return strings.HasSuffix(err.Error(), "certificate required") ||
					strings.HasSuffix(err.Error(), "alert(116)")
			}
			defer cls()
			return false
		}, e2e.WaitDuration, e2e.TickDuration)
	})
	suite.Run("correct_client_cert", func() {
		suite.client.Transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
			Certificates:       []tls.Certificate{suite.validClientCert},
		}
		suite.Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			// default backend
			return res.StatusCode == 404
		}, e2e.WaitDuration, e2e.TickDuration)
	})
	suite.Run("wrong_client_cert", func() {
		suite.client.Transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
			Certificates:       []tls.Certificate{suite.wrongClientCert},
		}
		suite.Eventually(func() bool {
			_, cls, err := suite.client.Do()
			if err != nil {
				return strings.HasSuffix(err.Error(), "certificate required") ||
					strings.HasSuffix(err.Error(), "alert(116)")
			}
			defer cls()
			return false
		}, e2e.WaitDuration, e2e.TickDuration)
	})
}
