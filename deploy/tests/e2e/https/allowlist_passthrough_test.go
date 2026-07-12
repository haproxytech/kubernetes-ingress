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

//go:build e2e_https

package https

import (
	"net/http"
	"testing"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

// AllowListWithClusterPassthroughSuite reproduces a regression where an
// unrelated ingress using ssl-passthrough elsewhere in the cluster flips the
// controller-wide haproxy.SSLPassthrough flag, which in turn caused
// allow-list/deny-list rules on a *different*, non-passthrough ingress to be
// dropped from the "https" frontend - the frontend where that ingress's own
// TLS-terminated traffic actually lands. The allow-list then went
// unenforced for that ingress's real traffic.
type AllowListWithClusterPassthroughSuite struct {
	HTTPSSuite
}

func TestAllowListWithClusterPassthroughSuite(t *testing.T) {
	suite.Run(t, new(AllowListWithClusterPassthroughSuite))
}

func (suite *AllowListWithClusterPassthroughSuite) Test_AllowList_Enforced_When_Another_Ingress_Uses_SSLPassthrough() {
	// A second, unrelated ingress using ssl-passthrough. Its mere presence in
	// the cluster flips haproxy.SSLPassthrough, which must not affect rule
	// placement for other, non-passthrough ingresses.
	defer suite.test.Delete("config/passthrough-sidecar-delete.yaml")
	passthroughData := tmplData{
		Host: "passthrough-sidecar." + suite.test.GetNS() + ".test",
		Port: "https",
		IngAnnotations: []struct{ Key, Value string }{
			{"ssl-passthrough", "'true'"},
		},
	}
	suite.Require().NoError(suite.test.Apply("config/passthrough-sidecar.yaml.tmpl", suite.test.GetNS(), passthroughData))

	// The ingress under test: allow-list only, no ssl-passthrough of its own.
	// suite.client (created in BeforeTest) targets suite.tmplData.Host over HTTPS.
	suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
		{"allow-list", "6.6.6.6"},
	}
	suite.Require().NoError(suite.test.Apply("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))

	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		// The test client is never 6.6.6.6, so its HTTPS request must be
		// denied - even though a coexisting ingress uses ssl-passthrough.
		return res.StatusCode == http.StatusForbidden
	}, e2e.WaitDuration, e2e.TickDuration, "expected HTTPS request to the non-passthrough ingress to be blocked by allow-list")
}
