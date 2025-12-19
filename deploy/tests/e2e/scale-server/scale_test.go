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

package scale

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

// Adding ScaleServerTestSuite, just to be able to debug directly here and not from ScaleServerSuite
type ScaleServerTestSuite struct {
	ScaleServerSuite
}

func TestScaleServerTestSuite(t *testing.T) {
	suite.Run(t, new(ScaleServerTestSuite))
}

func (suite *ScaleServerTestSuite) Test_ScaleServer() {
	var err error
	suite.tmplData.Replicas = 1
	suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Require().Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		return res.StatusCode == 200
	}, e2e.WaitDuration, e2e.TickDuration)
	suite.WaitForUpServersOnRuntime(fmt.Sprintf("%s_svc_http-echo_http", suite.test.GetNS()), 2)

	// Check Pid
	oldInfo, err := e2e.GetGlobalHAProxyInfo()
	suite.Require().NoError(err)

	//---------------------------------
	// Now scale up to 6 replicas (6 > number of slots (4))
	// This will create new sever slots
	suite.tmplData.Replicas = 6
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Require().Eventually(func() bool {
		out, err := suite.execute("", "kubectl", "-n", suite.test.GetNS(), "get", "deploy", "http-echo")
		suite.Require().NoError(err)
		return strings.Contains(out, "6/6")
	}, e2e.WaitDuration, e2e.TickDuration)
	suite.WaitForUpServersOnRuntime(fmt.Sprintf("%s_svc_http-echo_http", suite.test.GetNS()), 12)

	// 	Check that no reload occurs !!!!!
	newInfo, err := e2e.GetGlobalHAProxyInfo()
	suite.Require().NoError(err)
	suite.Require().Equal(oldInfo.Pid, newInfo.Pid)
}

func (suite *ScaleServerTestSuite) Test_ScaleServer_WithBackendCRD_NoReload() {
	var err error
	suite.Require().NoError(err)

	suite.tmplData.Replicas = 1
	suite.tmplData.BackendCR = true

	suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.test.Apply("config/backend.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Require().Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		return res.StatusCode == 200
	}, e2e.WaitDuration, e2e.TickDuration)
	suite.WaitForUpServersOnRuntime(fmt.Sprintf("%s_svc_http-echo_http", suite.test.GetNS()), 2)

	// Check Pid
	oldInfo, err := e2e.GetGlobalHAProxyInfo()

	//---------------------------------
	// Now scale up to 6 replicas (6 > number of slots (4))
	// This will create new sever slots
	suite.tmplData.Replicas = 6

	suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Require().Eventually(func() bool {
		out, err := suite.execute("", "kubectl", "-n", suite.test.GetNS(), "get", "deploy", "http-echo")
		suite.Require().NoError(err)
		return strings.Contains(out, "6/6")
	}, e2e.WaitDuration, e2e.TickDuration)
	suite.WaitForUpServersOnRuntime(fmt.Sprintf("%s_svc_http-echo_http", suite.test.GetNS()), 12)

	// 	Check that no reload occurs !!!!!
	newInfo, err := e2e.GetGlobalHAProxyInfo()
	suite.Require().NoError(err)
	suite.Require().Equal(oldInfo.Pid, newInfo.Pid)
}

// Same e2e test but with a backend CRD with an option that is REJECTED on the runtime server
// log-bufsize
// This should not trigger a reload (error on runtime `add server`)
func (suite *ScaleServerTestSuite) Test_ScaleServer_WithBackendCRD_Reload() {
	var err error
	suite.Require().NoError(err)

	startInfo, err := e2e.GetGlobalHAProxyInfo()
	suite.T().Logf("startInfo.Pid(%s)", startInfo.Pid)

	suite.tmplData.Replicas = 1
	suite.tmplData.BackendCR = true
	suite.tmplData.BackendCRWithReload = true

	suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.test.Apply("config/backend.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Require().Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		return res.StatusCode == 200
	}, e2e.WaitDuration, e2e.TickDuration)
	suite.WaitForUpServersOnRuntime(fmt.Sprintf("%s_svc_http-echo_http", suite.test.GetNS()), 2)

	oldInfo, err := e2e.GetGlobalHAProxyInfo()
	suite.T().Logf("oldInfo.Pid(%s)", oldInfo.Pid)

	//---------------------------------
	// Now scale up to 6 replicas (6 > number of slots (4))
	// This will create new sever slots
	suite.tmplData.Replicas = 6

	suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Require().Eventually(func() bool {
		out, err := suite.execute("", "kubectl", "-n", suite.test.GetNS(), "get", "deploy", "http-echo")
		suite.Require().NoError(err)
		return strings.Contains(out, "6/6")
	}, e2e.WaitDuration, e2e.TickDuration)
	suite.WaitForUpServersOnRuntime(fmt.Sprintf("%s_svc_http-echo_http", suite.test.GetNS()), 12)

	var newInfo e2e.GlobalHAProxyInfo
	var errNew error
	// 	Check that reload occurs !!!!!
	suite.Require().Eventually(func() bool {
		newInfo, errNew = e2e.GetGlobalHAProxyInfo()
		suite.Require().NoError(errNew)
		suite.T().Logf("newInfo.Pid(%s)", newInfo.Pid)
		return oldInfo.Pid != newInfo.Pid
	}, e2e.WaitDuration, e2e.TickDuration, fmt.Sprintf("Reload should have occurred: oldInfo.Pid(%s) != newInfo.Pid(%s)", oldInfo.Pid, newInfo.Pid))
}

func (t *ScaleServerTestSuite) execute(entry, command string, args ...string) (string, error) { //nolint: unparam
	cmd := exec.Command(command, args...)
	var b bytes.Buffer
	b.WriteString(entry)
	cmd.Stdin = &b
	output, err := cmd.CombinedOutput()
	return string(output), err
}
