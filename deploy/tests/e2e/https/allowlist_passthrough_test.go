// Copyright 2026 HAProxy Technologies LLC
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
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

// Adding AllowListWithClusterPassthroughSuite, just to be able to debug directly here and not from CRDSuite
type AllowListWithClusterPassthroughSuite struct {
	HTTPSSuite
}

func TestAllowListWithClusterPassthroughSuite(t *testing.T) {
	suite.Run(t, new(AllowListWithClusterPassthroughSuite))
}

func (suite *AllowListWithClusterPassthroughSuite) Test_AllowList_With_ClusterPassthrough() {
	// A second, unrelated ingress using ssl-passthrough. Its mere presence in
	// the cluster flips haproxy.SSLPassthrough, which must not affect rule
	// placement for other, non-passthrough ingresses.
	defer func() {
		suite.Require().NoError(suite.test.Delete("config/passthrough-sidecar-delete.yaml"))
	}()
	sidecarHost := "passthrough-sidecar." + suite.test.GetNS() + ".test"
	passthroughData := tmplData{
		Host: sidecarHost,
		Port: "https",
		IngAnnotations: []struct{ Key, Value string }{
			{"ssl-passthrough", "'true'"},
		},
	}
	suite.Require().NoError(suite.test.Apply("config/passthrough-sidecar.yaml.tmpl", suite.test.GetNS(), passthroughData))

	// Confirm the sidecar's ssl-passthrough is genuinely active before relying
	// on it as this test's precondition - mirrors passthrough_test.go's own
	// Reach_Backend check. The echo backend only reports a TLS SNI when
	// HAProxy relayed a raw passthrough connection instead of terminating
	// TLS itself, so this can't pass unless SSLPassthrough actually flipped.
	sidecarClient, err := e2e.NewHTTPSClient(sidecarHost)
	suite.Require().NoError(err)
	suite.Eventually(func() bool {
		res, cls, err := sidecarClient.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return false
		}
		type echoServerResponse struct {
			TLS struct {
				SNI string `json:"sni"`
			} `json:"tls"`
		}
		response := &echoServerResponse{}
		if err := json.Unmarshal(body, response); err != nil {
			return false
		}
		return response.TLS.SNI == sidecarHost
	}, e2e.WaitDuration, e2e.TickDuration, "expected the sidecar ssl-passthrough ingress to be active before testing the main ingress's allow-list")

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

	// Positive control on the same ingress/host/frontend: widen the allow-list
	// to everyone and confirm the same client now gets through. FrontHTTPS is
	// shared by every TLS-terminated ingress in the cluster, so proving the
	// prior 403 alone doesn't rule out a regression that makes the deny
	// unconditional (denies everyone) rather than correctly scoped.
	suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
		{"allow-list", "0.0.0.0/0"},
	}
	suite.Require().NoError(suite.test.Apply("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))

	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if res == nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		return res.StatusCode == http.StatusOK
	}, e2e.WaitDuration, e2e.TickDuration, "expected HTTPS request to succeed once the allow-list is widened to everyone")
}
