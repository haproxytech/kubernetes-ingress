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

package crd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type RejectedGlobalSuite struct {
	CRDSuite
}

func TestRejectedGlobalSuite(t *testing.T) {
	suite.Run(t, new(RejectedGlobalSuite))
}

// Test_CR_Global_Rejected: the configmap of the test deployment references the
// Global resource haproxy-controller/global-full; a Lua file that does not exist
// makes HAProxy refuse the whole configuration at check time. The controller must
// set the resource aside, build the global section from the configmap annotations,
// keep applying unrelated changes, and pick the resource up again once edited.
func (suite *RejectedGlobalSuite) Test_CR_Global_Rejected() {
	globalCRPath := "config/global-rejected.yaml.tmpl"
	secondIngressPath := "config/ingress-second-host.yaml.tmpl"
	controllerNS := "haproxy-controller"

	// ---- Phase 1: a resource HAProxy refuses at configuration check --------
	suite.tmplData.GlobalDescription = "rejected"
	suite.tmplData.GlobalLuaFile = "/nonexistent.lua"

	suite.Require().NoError(suite.test.Apply(globalCRPath, controllerNS, suite.tmplData))
	suite.test.AddTearDown(func() error {
		return suite.test.DeleteInNamespace(globalCRPath, controllerNS, suite.tmplData)
	})

	// An unrelated change must still be applied while the resource is disabled.
	suite.Require().NoError(suite.test.Apply(secondIngressPath, suite.test.GetNS(), suite.tmplData))
	suite.test.AddTearDown(func() error {
		return suite.test.Delete(secondIngressPath)
	})
	secondClient, err := e2e.NewHTTPClient(suite.tmplData.SecondHost)
	suite.Require().NoError(err)
	suite.Require().Eventually(func() bool {
		r, cls, err := secondClient.Do()
		if err != nil {
			return false
		}
		defer cls()
		return r.StatusCode == 200
	}, e2e.WaitDuration, e2e.TickDuration, "phase 1: a change made after the rejected resource never reached HAProxy")

	cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err)
	suite.Require().NotContains(cfg, "lua-load", "phase 1: the rejected directive must not be rendered")
	suite.Require().NotContains(cfg, "description rejected", "phase 1: the whole resource is set aside, not just the rejected directive")

	// ---- Phase 2: the edited resource is retried -------------------------
	suite.tmplData.GlobalDescription = "fixed"
	suite.tmplData.GlobalLuaFile = ""

	suite.Require().NoError(suite.test.Apply(globalCRPath, controllerNS, suite.tmplData))

	suite.Require().Eventually(func() bool {
		cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
		return err == nil && strings.Contains(cfg, "description fixed")
	}, e2e.WaitDuration, e2e.TickDuration, "phase 2: the edited resource was not applied")
}
