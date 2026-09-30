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
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	parser "github.com/haproxytech/client-native/v6/config-parser"
	"github.com/haproxytech/client-native/v6/config-parser/options"
	"github.com/haproxytech/client-native/v6/config-parser/types"

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
//   server s<hash> 10.244.0.9:8888 enabled
//   ...

func (suite *CookiePersistenceTestSuite) Test_CookiePersistence_Dynamic() {
	//------------------------
	// First step : Dynamic
	suite.resetTemplateData()
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
	names := suite.configServerNames(p, beName)
	suite.Require().Len(names, 1)
	serverName := names[0]

	err = suite.checkServerNoCookie(p, beName, serverName)
	suite.Require().NoError(err, "check server no cookie")

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

	err = suite.checkServerNoCookie(p, beName, serverName)
	suite.Require().NoError(err, "check server no cookie")
}

// Expected backend
// backend e2e-tests-cookie-persistence_svc_http-echo_http
//   ...
//   cookie mycookie indirect nocache insert
//   server s<hash> 10.244.0.13:8888 enabled cookie s<hash>
//   ...

func (suite *CookiePersistenceTestSuite) Test_CookiePersistence_No_Dynamic() {
	suite.resetTemplateData()
	suite.tmplData.CookiePersistenceNoDynamic = true
	suite.tmplData.CookiePersistenceDynamic = false
	suite.Require().NoError(suite.test.Apply("config/deploy.yml.tmpl", suite.test.GetNS(), suite.tmplData))
	// Check that curl backend return 200 and "Set-Cookie ""mycookie=<SRV_NAME>; path=/"
	var pinnedServer string
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		pinnedServer = cookieServer(res.Header["Set-Cookie"])

		return res.StatusCode == http.StatusOK && isServerCookie(pinnedServer)
	}, e2e.WaitDuration, e2e.TickDuration)

	// Also check configuration
	cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err, "Could not get Haproxy config")

	suite.Require().Contains(cfg, "cookie mycookie indirect nocache insert") // NOTE that it does not contains dynamic
	suite.Require().NotContains(cfg, "dynamic-cookie-key")

	reader := strings.NewReader(cfg)
	p, err := parser.New(options.Reader(reader))
	suite.Require().NoError(err, "Could not get Haproxy config parser")

	// Check the server line
	beName := suite.test.GetNS() + "_svc_http-echo_http"
	names := suite.configServerNames(p, beName)
	suite.Require().Len(names, 1)
	serverName := names[0]
	suite.Require().Equal(serverName, pinnedServer, "the cookie must name the configured server")

	suite.checkServerCookie(p, beName, serverName)

	// Scale up: the new servers are created through the runtime and must carry a cookie value too.
	suite.tmplData.Replicas = 3
	suite.Require().NoError(suite.test.Apply("config/deploy.yml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Require().Eventually(func() bool {
		count, err := e2e.GetRuntimeUpServersCount(beName)
		return err == nil && count == 3
	}, e2e.WaitDuration, e2e.TickDuration, "3 servers should be up")

	expectedServers := suite.waitForConfigServers(beName, 3)

	seen := map[string]bool{}
	suite.Require().Eventually(func() bool {
		for range 30 {
			res, cls, err := suite.client.Do()
			if err != nil {
				suite.T().Log(err)
				return false
			}
			value := cookieServer(res.Header.Values("Set-Cookie"))
			suite.Require().NoError(cls())
			suite.Require().Contains(expectedServers, value, "cookie must name a configured server")
			seen[value] = true
		}
		return len(seen) == 3
	}, e2e.WaitDuration, e2e.TickDuration, "round robin must have pinned to every server, including runtime-created ones")

	// Scale back down and wait for the config to follow, so the next steps see one server.
	suite.tmplData.Replicas = 1
	suite.Require().NoError(suite.test.Apply("config/deploy.yml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.waitForConfigServers(beName, 1)

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

	err = suite.checkServerNoCookie(p, beName, serverName)
	suite.Require().NoError(err, "check server no cookie")
}

func (suite *CookiePersistenceTestSuite) Test_CookiePersistence_Switch() {
	//---------------------------
	// Step 1 : Dynamic
	suite.resetTemplateData()
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
	names := suite.configServerNames(p, beName)
	suite.Require().Len(names, 1)
	serverName := names[0]

	err = suite.checkServerNoCookie(p, beName, serverName)
	suite.Require().NoError(err, "check server no cookie")

	//----------------------
	// Step 2: not dynamic
	suite.tmplData.CookiePersistenceNoDynamic = true
	suite.tmplData.CookiePersistenceDynamic = false
	suite.Require().NoError(suite.test.Apply("config/deploy.yml.tmpl", suite.test.GetNS(), suite.tmplData))
	// Check that curl backend return 200 and "Set-Cookie ""mycookie=<SRV_NAME>; path=/"
	var pinnedServer string
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		pinnedServer = cookieServer(res.Header["Set-Cookie"])

		return res.StatusCode == http.StatusOK && isServerCookie(pinnedServer)
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
	suite.Require().Equal(serverName, pinnedServer, "the cookie must name the configured server")
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
	suite.Eventually(func() bool {
		cfg, err = suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
		if err != nil {
			suite.T().Logf("Could not get Haproxy config: %v", err)
			return false
		}
		if !strings.Contains(cfg, "cookie mycookie dynamic indirect nocache insert") {
			return false
		}
		if !strings.Contains(cfg, "dynamic-cookie-key") {
			return false
		}
		// Check that the server line does not contain "cookie" param
		reader = strings.NewReader(cfg)
		p, err = parser.New(options.Reader(reader))
		if err != nil {
			suite.T().Logf("Could not get Haproxy config parser: %v", err)
			return false
		}
		err = suite.checkServerNoCookie(p, beName, serverName)
		if err != nil {
			suite.T().Logf("check server no cookie: %v", err)
			return false
		}

		return true
	}, e2e.WaitDuration, e2e.TickDuration)
}

// configServerNames returns the server names haproxy.cfg declares for the backend.
func (suite *CookiePersistenceTestSuite) configServerNames(p parser.Parser, beName string) []string {
	data, err := p.Get("backend", beName, "server", false)
	suite.Require().NoError(err, "Could not get backend servers")
	servers, ok := data.([]types.Server)
	suite.Require().True(ok, "Could not get backend servers")
	names := make([]string, 0, len(servers))
	for _, s := range servers {
		names = append(names, s.Name)
	}
	return names
}

// waitForConfigServers polls haproxy.cfg until the backend declares exactly n servers.
func (suite *CookiePersistenceTestSuite) waitForConfigServers(beName string, n int) []string {
	var names []string
	suite.Require().Eventually(func() bool {
		cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
		if err != nil {
			return false
		}
		p, err := parser.New(options.Reader(strings.NewReader(cfg)))
		if err != nil {
			return false
		}
		data, err := p.Get("backend", beName, "server", false)
		servers, ok := data.([]types.Server)
		if err != nil || !ok || len(servers) != n {
			return false
		}
		names = names[:0]
		for _, s := range servers {
			names = append(names, s.Name)
		}
		return true
	}, e2e.WaitDuration, e2e.TickDuration, fmt.Sprintf("config should declare %d servers", n))
	return names
}

// Server names start with "s"; dynamic cookie values are plain hex.
var serverCookieRe = regexp.MustCompile(`^s[0-9a-f]+$`)

func isServerCookie(value string) bool {
	return serverCookieRe.MatchString(value)
}

// cookieServer extracts the server name carried by mycookie, "" if absent.
func cookieServer(cookies []string) string {
	for _, cookie := range cookies {
		for _, part := range strings.Split(cookie, ";") {
			if name, value, found := strings.Cut(strings.TrimSpace(part), "="); found && name == "mycookie" {
				return value
			}
		}
	}
	return ""
}

// Expected backend
// backend e2e-tests-cookie-persistence_svc_http-echo_http
//
//	...
//	balance source
//	stick-table type ip size 1m expire 30m peers localinstance
//	stick on src
//	...
func (suite *CookiePersistenceTestSuite) Test_SourceIPPersistence_WithSourceHash() {
	suite.resetTemplateData()
	suite.tmplData.Replicas = 3
	suite.tmplData.SourceIPPersistence = true
	suite.tmplData.LoadBalance = "source"
	suite.Require().NoError(suite.test.Apply("config/deploy.yml.tmpl", suite.test.GetNS(), suite.tmplData))

	var firstHostname string
	suite.Eventually(func() bool {
		hostname, ok := suite.echoHostname()
		if !ok {
			return false
		}
		firstHostname = hostname
		return true
	}, e2e.WaitDuration, e2e.TickDuration)

	for range 10 {
		hostname, ok := suite.echoHostname()
		suite.Require().True(ok)
		suite.Require().Equal(firstHostname, hostname)
	}

	suite.Eventually(func() bool {
		cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
		if err != nil {
			suite.T().Logf("Could not get Haproxy config: %v", err)
			return false
		}
		return suite.hasSourceIPPersistenceConfig(cfg)
	}, e2e.WaitDuration, e2e.TickDuration)
}

func (suite *CookiePersistenceTestSuite) hasSourceIPPersistenceConfig(cfg string) bool {
	if !strings.Contains(cfg, "balance source") || !strings.Contains(cfg, "stick on src") {
		return false
	}
	reader := strings.NewReader(cfg)
	p, err := parser.New(options.Reader(reader))
	if err != nil {
		suite.T().Logf("Could not get Haproxy config parser: %v", err)
		return false
	}
	beName := suite.test.GetNS() + "_svc_http-echo_http"
	rawTable, err := p.Get(parser.Backends, beName, "stick-table")
	if err != nil {
		suite.T().Logf("Could not get stick-table for backend %s: %v", beName, err)
		return false
	}
	table, ok := rawTable.(*types.StickTable)
	if !ok {
		suite.T().Logf("Unexpected stick-table type %T", rawTable)
		return false
	}
	return table.Type == "ip" &&
		table.Size == "1m" &&
		(table.Expire == "30m" || table.Expire == "1800000") &&
		table.Peers == "localinstance"
}

func (suite *CookiePersistenceTestSuite) echoHostname() (string, bool) {
	res, cls, err := suite.client.Do()
	if res == nil {
		suite.T().Log(err)
		return "", false
	}
	defer cls()
	if res.StatusCode != http.StatusOK {
		return "", false
	}
	var body struct {
		OS struct {
			Hostname string `json:"hostname"`
		} `json:"os"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		suite.T().Log(err)
		return "", false
	}
	return body.OS.Hostname, body.OS.Hostname != ""
}
