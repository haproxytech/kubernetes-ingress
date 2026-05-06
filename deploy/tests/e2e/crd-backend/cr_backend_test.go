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

package crdbackend

import (
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

type BackendSuite struct {
	CRDBackendSuite
}

func TestBackendSuite(t *testing.T) {
	suite.Run(t, new(BackendSuite))
}

// Test_CR_Backend_RuleLists exercises the rule-list fields the controller now
// populates onto backend sections (the fixes that mirror ebc61be8 across all
// list-typed Backend CRD sub-resources).
//
// First phase applies a CR carrying every rule list and asserts each directive
// shows up in the rendered haproxy.cfg under the Backend's chosen section name.
//
// Second phase changes ONLY the rule-list contents (BackendBase fields stay
// identical) and asserts the new values appear and old values are gone — this
// is the bug-2 reload check from ebc61be8.
func (suite *BackendSuite) Test_CR_Backend_RuleLists() {
	backendCRPath := "config/backend.yaml.tmpl"

	// ---- Phase 1: initial rule lists ------------------------------------
	suite.tmplData.HeaderName = "X-Phase"
	suite.tmplData.HeaderValue = "first"
	suite.tmplData.HealthURI = "/healthz"
	suite.tmplData.LogAddress = "127.0.0.1:514"

	suite.Require().NoError(suite.test.Apply(backendCRPath, suite.test.GetNS(), suite.tmplData))
	suite.test.AddTearDown(func() error {
		return suite.test.Delete(backendCRPath)
	})

	suite.Require().Eventually(func() bool {
		b, err := suite.getBackend(suite.tmplData.BackendName)
		if err != nil || b == nil {
			return false
		}
		return ruleListsMatch(b, "X-Phase", "first", "/healthz", "127.0.0.1:514")
	}, e2e.WaitDuration, e2e.TickDuration, "phase 1: rule lists never appeared in rendered backend")

	// ---- Phase 2: change ONLY the rule-list contents --------------------
	suite.tmplData.HeaderName = "X-Phase"
	suite.tmplData.HeaderValue = "second"
	suite.tmplData.HealthURI = "/ready"
	suite.tmplData.LogAddress = "10.0.0.1:5000"

	suite.Require().NoError(suite.test.Apply(backendCRPath, suite.test.GetNS(), suite.tmplData))

	suite.Require().Eventually(func() bool {
		b, err := suite.getBackend(suite.tmplData.BackendName)
		if err != nil || b == nil {
			return false
		}
		// New values present AND old values gone — this verifies that
		// rule-list-only changes trigger a reload (the bug-2 fix).
		if !ruleListsMatch(b, "X-Phase", "second", "/ready", "10.0.0.1:5000") {
			return false
		}
		return !hasHeaderRule(b.HTTPRequestRuleList, "X-Phase", "first") &&
			!hasHTTPCheckURI(b.HTTPCheckList, "/healthz") &&
			!hasLogAddress(b.LogTargetList, "127.0.0.1:514")
	}, e2e.WaitDuration, e2e.TickDuration, "phase 2: rule-list-only update did not propagate")
}

// ruleListsMatch checks that the populated backend contains the expected
// directives across every rule-list field we wired up.
func ruleListsMatch(b *models.Backend, hdrName, hdrValue, healthURI, logAddr string) bool {
	if !hasACL(b.ACLList, "is_admin") {
		return false
	}
	if !hasHeaderRule(b.HTTPRequestRuleList, hdrName, hdrValue) {
		return false
	}
	if !hasResponseHeader(b.HTTPResponseRuleList, "X-Resp-Test", hdrValue) {
		return false
	}
	if !hasAfterResponseHeader(b.HTTPAfterResponseRuleList, "X-After-Test", hdrValue) {
		return false
	}
	if !hasHTTPCheckURI(b.HTTPCheckList, healthURI) {
		return false
	}
	if !hasLogAddress(b.LogTargetList, logAddr) {
		return false
	}
	if !hasFilterType(b.FilterList, "compression") {
		return false
	}
	if !hasUseServer(b.ServerSwitchingRuleList, "SRV_1") {
		return false
	}
	if !hasStickPattern(b.StickRuleList, "src") {
		return false
	}
	if !hasTCPRequestAcceptCond(b.TCPRequestRuleList) {
		return false
	}
	return true
}

func hasACL(list models.Acls, name string) bool {
	for _, a := range list {
		if a != nil && a.ACLName == name {
			return true
		}
	}
	return false
}

func hasHeaderRule(list models.HTTPRequestRules, name, value string) bool {
	for _, r := range list {
		if r != nil && r.Type == "set-header" && r.HdrName == name && r.HdrFormat == value {
			return true
		}
	}
	return false
}

func hasResponseHeader(list models.HTTPResponseRules, name, value string) bool {
	for _, r := range list {
		if r != nil && r.Type == "set-header" && r.HdrName == name && r.HdrFormat == value {
			return true
		}
	}
	return false
}

func hasAfterResponseHeader(list models.HTTPAfterResponseRules, name, value string) bool {
	for _, r := range list {
		if r != nil && r.Type == "set-header" && r.HdrName == name && r.HdrFormat == value {
			return true
		}
	}
	return false
}

func hasHTTPCheckURI(list models.HTTPChecks, uri string) bool {
	for _, c := range list {
		if c != nil && c.Type == "send" && c.URI == uri {
			return true
		}
	}
	return false
}

func hasLogAddress(list models.LogTargets, addr string) bool {
	for _, t := range list {
		if t != nil && t.Address == addr {
			return true
		}
	}
	return false
}

func hasFilterType(list models.Filters, t string) bool {
	for _, f := range list {
		if f != nil && f.Type == t {
			return true
		}
	}
	return false
}

func hasUseServer(list models.ServerSwitchingRules, target string) bool {
	for _, r := range list {
		if r != nil && r.TargetServer == target {
			return true
		}
	}
	return false
}

func hasStickPattern(list models.StickRules, pattern string) bool {
	for _, r := range list {
		if r != nil && r.Pattern == pattern {
			return true
		}
	}
	return false
}

func hasTCPRequestAcceptCond(list models.TCPRequestRules) bool {
	for _, r := range list {
		if r != nil && r.Type == "content" && r.Action == "accept" && r.Cond == "if" {
			return true
		}
	}
	return false
}
