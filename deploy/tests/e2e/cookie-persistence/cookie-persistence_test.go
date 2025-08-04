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

package cookiepersistence

import (
	"net/http"
	"strings"
	"testing"

	parser "github.com/haproxytech/client-native/v5/config-parser"
	"github.com/haproxytech/client-native/v5/config-parser/options"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

// Adding CookiePersistenceTest, just to be able to debug directly here and not from CRDTCPSuite
type CookiePersistenceTestSuite struct {
	CookiePersistenceSuite
}

func TestCookiePersistenceTestSuite(t *testing.T) {
	suite.Run(t, new(CookiePersistenceTestSuite))
}

// Expected backend
// backend e2e-tests-cookie-persistence_svc_http-echo_http
//   ...
//   cookie mycookie dynamic indirect nocache insert
//   dynamic-cookie-key ohph7OoGhong
//   server SRV_1 10.244.0.9:8888 enabled
//   ...

func (suite *CookiePersistenceTestSuite) Test_CookiePersistence_Dynamic() {
	//------------------------
	// First step : Dynamic
	suite.tmplData.CookiePersistenceDynamic = true
	suite.tmplData.CookiePersistenceNoDynamic = false
	suite.Require().NoError(suite.test.Apply("config/deploy.yml.tmpl", suite.test.GetNS(), suite.tmplData))
	// Check that curl backend return 200 and "Set-Cookie ""mycookie=f8f1bc84b3d0d5c0; path=/"
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		cookies := res.Header["Set-Cookie"]
		cookieOK := false
		if len(cookies) != 0 {
			for _, cookie := range cookies {
				if strings.Contains(cookie, "mycookie") {
					cookieOK = true
					break
				}
			}
		}

		return res.StatusCode == http.StatusOK && cookieOK
	}, e2e.WaitDuration, e2e.TickDuration)

	// Also check configuration
	cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err, "Could not get Haproxy config")

	suite.Require().Contains(cfg, "cookie mycookie dynamic indirect nocache insert")
	suite.Require().Contains(cfg, "dynamic-cookie-key")

	// Check that the server line does not contain "cookie" param
	reader := strings.NewReader(cfg)
	p, err := parser.New(options.Reader(reader))
	suite.Require().NoError(err, "Could not get Haproxy config parser")
	beName := suite.test.GetNS() + "_svc_http-echo_http"
	serverName := "SRV_1"

	suite.checkServerNoCookie(p, beName, serverName)

	// ------------------------
	// Second step : remove annotation
	suite.tmplData.CookiePersistenceDynamic = false
	suite.tmplData.CookiePersistenceNoDynamic = false
	suite.Require().NoError(suite.test.Apply("config/deploy.yml.tmpl", suite.test.GetNS(), suite.tmplData))
	// Check that curl backend return 200 and "Set-Cookie ""mycookie=f8f1bc84b3d0d5c0; path=/"
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		_, cookieOK := res.Header["Set-Cookie"]

		return res.StatusCode == http.StatusOK && !cookieOK
	}, e2e.WaitDuration, e2e.TickDuration)

	// Also check configuration
	cfg, err = suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err, "Could not get Haproxy config")

	suite.Require().NotContains(cfg, "cookie mycookie dynamic indirect nocache insert")
	suite.Require().NotContains(cfg, "dynamic-cookie-key")
	// Check that the server line does not contain "cookie" param
	reader = strings.NewReader(cfg)
	p, err = parser.New(options.Reader(reader))
	suite.Require().NoError(err, "Could not get Haproxy config parser")

	suite.checkServerNoCookie(p, beName, serverName)
}

// Expected backend
// backend e2e-tests-cookie-persistence_svc_http-echo_http
//   ...
//   cookie mycookie indirect nocache insert
//   server SRV_1 10.244.0.13:8888 enabled cookie SRV_1
//   ...

func (suite *CookiePersistenceTestSuite) Test_CookiePersistence_No_Dynamic() {
	suite.tmplData.CookiePersistenceNoDynamic = true
	suite.tmplData.CookiePersistenceDynamic = false
	suite.Require().NoError(suite.test.Apply("config/deploy.yml.tmpl", suite.test.GetNS(), suite.tmplData))
	// Check that curl backend return 200 and "Set-Cookie ""mycookie=<SRV_NAME>; path=/"
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		cookies := res.Header["Set-Cookie"]
		cookieOK := false
		if len(cookies) != 0 {
			for _, cookie := range cookies {
				if strings.Contains(cookie, "mycookie") && strings.Contains(cookie, "SRV_1") {
					cookieOK = true
					break
				}
			}
		}

		return res.StatusCode == http.StatusOK && cookieOK
	}, e2e.WaitDuration, e2e.TickDuration)

	// Also check configuration
	cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err, "Could not get Haproxy config")

	suite.Require().Contains(cfg, "cookie mycookie indirect nocache insert") // NOTE that it does not contains dynamic
	suite.Require().NotContains(cfg, "dynamic-cookie-key")

	reader := strings.NewReader(cfg)
	p, err := parser.New(options.Reader(reader))
	suite.Require().NoError(err, "Could not get Haproxy config parser")

	// Check that the server line
	beName := suite.test.GetNS() + "_svc_http-echo_http"
	serverName := "SRV_1"

	suite.checkServerCookie(p, beName, serverName)

	// ------------------------
	// Second step : remove annotation
	suite.tmplData.CookiePersistenceDynamic = false
	suite.tmplData.CookiePersistenceNoDynamic = false
	suite.Require().NoError(suite.test.Apply("config/deploy.yml.tmpl", suite.test.GetNS(), suite.tmplData))
	// Check that curl backend return 200 and "Set-Cookie ""mycookie=f8f1bc84b3d0d5c0; path=/"
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		_, cookieOK := res.Header["Set-Cookie"]

		return res.StatusCode == http.StatusOK && !cookieOK
	}, e2e.WaitDuration, e2e.TickDuration)

	// Also check configuration
	cfg, err = suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err, "Could not get Haproxy config")

	suite.Require().NotContains(cfg, "cookie mycookie dynamic indirect nocache insert")
	suite.Require().NotContains(cfg, "dynamic-cookie-key")
	// Check that the server line does not contain "cookie" param
	reader = strings.NewReader(cfg)
	p, err = parser.New(options.Reader(reader))
	suite.Require().NoError(err, "Could not get Haproxy config parser")

	suite.checkServerNoCookie(p, beName, serverName)
}

func (suite *CookiePersistenceTestSuite) Test_CookiePersistence_Switch() {
	//---------------------------
	// Step 1 : Dynamic
	suite.tmplData.CookiePersistenceDynamic = true
	suite.tmplData.CookiePersistenceNoDynamic = false
	suite.Require().NoError(suite.test.Apply("config/deploy.yml.tmpl", suite.test.GetNS(), suite.tmplData))
	// Check that curl backend return 200 and "Set-Cookie ""mycookie=f8f1bc84b3d0d5c0; path=/"
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		cookies := res.Header["Set-Cookie"]
		cookieOK := false
		if len(cookies) != 0 {
			for _, cookie := range cookies {
				if strings.Contains(cookie, "mycookie") {
					cookieOK = true
					break
				}
			}
		}

		return res.StatusCode == http.StatusOK && cookieOK
	}, e2e.WaitDuration, e2e.TickDuration)

	// Also check configuration
	cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err, "Could not get Haproxy config")

	suite.Require().Contains(cfg, "cookie mycookie dynamic indirect nocache insert")
	suite.Require().Contains(cfg, "dynamic-cookie-key")

	// Check that the server line does not contain "cookie" param
	reader := strings.NewReader(cfg)
	p, err := parser.New(options.Reader(reader))
	suite.Require().NoError(err, "Could not get Haproxy config parser")
	beName := suite.test.GetNS() + "_svc_http-echo_http"
	serverName := "SRV_1"

	suite.checkServerNoCookie(p, beName, serverName)

	//----------------------
	// Step 2: not dynamic
	suite.tmplData.CookiePersistenceNoDynamic = true
	suite.tmplData.CookiePersistenceDynamic = false
	suite.Require().NoError(suite.test.Apply("config/deploy.yml.tmpl", suite.test.GetNS(), suite.tmplData))
	// Check that curl backend return 200 and "Set-Cookie ""mycookie=<SRV_NAME>; path=/"
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		cookies := res.Header["Set-Cookie"]
		cookieOK := false
		if len(cookies) != 0 {
			for _, cookie := range cookies {
				if strings.Contains(cookie, "mycookie") && strings.Contains(cookie, "SRV_1") {
					cookieOK = true
					break
				}
			}
		}

		return res.StatusCode == http.StatusOK && cookieOK
	}, e2e.WaitDuration, e2e.TickDuration)

	// Also check configuration
	cfg, err = suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err, "Could not get Haproxy config")

	suite.Require().Contains(cfg, "cookie mycookie indirect nocache insert") // NOTE that it does not contains dynamic
	suite.Require().NotContains(cfg, "dynamic-cookie-key")

	reader = strings.NewReader(cfg)
	p, err = parser.New(options.Reader(reader))
	suite.Require().NoError(err, "Could not get Haproxy config parser")

	// Check that the server line
	suite.checkServerCookie(p, beName, serverName)

	//------------------------
	// Step 3: and back: Dynamic
	suite.tmplData.CookiePersistenceDynamic = true
	suite.tmplData.CookiePersistenceNoDynamic = false
	suite.Require().NoError(suite.test.Apply("config/deploy.yml.tmpl", suite.test.GetNS(), suite.tmplData))
	// Check that curl backend return 200 and "Set-Cookie ""mycookie=f8f1bc84b3d0d5c0; path=/"
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		cookies := res.Header["Set-Cookie"]
		cookieOK := false
		if len(cookies) != 0 {
			for _, cookie := range cookies {
				if strings.Contains(cookie, "mycookie") {
					cookieOK = true
					break
				}
			}
		}

		return res.StatusCode == http.StatusOK && cookieOK
	}, e2e.WaitDuration, e2e.TickDuration)

	// Also check configuration
	cfg, err = suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err, "Could not get Haproxy config")

	suite.Require().Contains(cfg, "cookie mycookie dynamic indirect nocache insert")
	suite.Require().Contains(cfg, "dynamic-cookie-key")

	// Check that the server line does not contain "cookie" param
	reader = strings.NewReader(cfg)
	p, err = parser.New(options.Reader(reader))
	suite.Require().NoError(err, "Could not get Haproxy config parser")

	suite.checkServerNoCookie(p, beName, serverName)
}
