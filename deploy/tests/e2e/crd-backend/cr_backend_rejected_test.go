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

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type RejectedBackendSuite struct {
	CRDBackendSuite
}

func TestRejectedBackendSuite(t *testing.T) {
	suite.Run(t, new(RejectedBackendSuite))
}

// Test_CR_Backend_Rejected: HAProxy checks the whole configuration at once, so a Backend custom resource it
// rejects used to block every later change until someone fixed it. The controller now
// disables the rejected generation and replays the sync without it: the backend is
// built from the annotations, and unrelated changes keep going through. Editing the
// resource puts it back under test.
func (suite *RejectedBackendSuite) Test_CR_Backend_Rejected() {
	backendCRPath := "config/backend-rejected.yaml.tmpl"
	secondIngressPath := "config/ingress-second-host.yaml.tmpl"

	// ---- Phase 1: a resource HAProxy refuses at configuration check --------
	suite.tmplData.HeaderName = "X-CR-State"
	suite.tmplData.HeaderValue = "rejected"
	suite.tmplData.ErrorFile = "/nonexistent-errorfile"

	suite.Require().NoError(suite.test.Apply(backendCRPath, suite.test.GetNS(), suite.tmplData))
	suite.test.AddTearDown(func() error {
		return suite.test.Delete(backendCRPath)
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

	b, err := suite.getBackend(suite.tmplData.BackendName)
	suite.Require().NoError(err)
	suite.Require().Empty(b.ErrorFiles, "phase 1: the rejected directive must not be rendered")
	suite.Require().False(hasHeaderRule(b.HTTPRequestRuleList, "X-CR-State", "rejected"),
		"phase 1: the whole resource is set aside, not just the rejected directive")

	// ---- Phase 2: the edited resource is retried -------------------------
	suite.tmplData.HeaderValue = "fixed"
	suite.tmplData.ErrorFile = ""

	suite.Require().NoError(suite.test.Apply(backendCRPath, suite.test.GetNS(), suite.tmplData))

	suite.Require().Eventually(func() bool {
		b, err := suite.getBackend(suite.tmplData.BackendName)
		if err != nil || b == nil {
			return false
		}
		return hasHeaderRule(b.HTTPRequestRuleList, "X-CR-State", "fixed")
	}, e2e.WaitDuration, e2e.TickDuration, "phase 2: the edited resource was not applied")
}
