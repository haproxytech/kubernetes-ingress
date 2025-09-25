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

package routeacl

import (
	"os"
	"path/filepath"
	"strings"
)

func (suite *UseBackendSuite) TestUseBackend() {
	// This test addresses https://github.com/haproxytech/kubernetes-ingress/issues/476
	suite.UseBackendFixture()
	suite.Run("Modifying service annotations should not duplicate use_backend clause", func() {
		contents, err := os.ReadFile(filepath.Join(suite.test.TempDir, "haproxy.cfg"))
		if err != nil {
			suite.T().Error(err.Error())
		}
		c := strings.Count(string(contents), "use_backend ns_svc_myappservice_https if { path -m beg / } { cookie(staging) -m found }")
		suite.Exactly(c, 2, "use_backend for route-acl is repeated %d times but expected 2", c)
	})
}

func (suite *UseBackendSuite) TestNonWildcardHostWithRouteACL() {
	// Test non-wildcard host first to ensure route-acl works
	suite.NonWildcardHostFixture()
	suite.Run("Non-wildcard host should use string matching (-m str) with route-acl", func() {
		contents, err := os.ReadFile(filepath.Join(suite.test.TempDir, "haproxy.cfg"))
		if err != nil {
			suite.T().Error(err.Error())
		}

		// Check that -m str is used with non-wildcard hosts in route-acl
		if !strings.Contains(string(contents), "var(txn.host) -m str api.example.local") {
			suite.T().Error("Expected to find 'var(txn.host) -m str api.example.local' in HAProxy config")
		}

		// Check that route-acl annotation is applied
		if !strings.Contains(string(contents), "path_reg path-in-bug-repro$") {
			suite.T().Error("Expected to find route-acl pattern 'path_reg path-in-bug-repro$' in HAProxy config")
		}
	})
}

func (suite *UseBackendSuite) TestWildcardHostWithRouteACL() {
	// This test addresses https://github.com/haproxytech/kubernetes-ingress/issues/734
	suite.WildcardHostFixture()
	suite.Run("Wildcard host should use suffix matching (-m end) with route-acl", func() {
		contents, err := os.ReadFile(filepath.Join(suite.test.TempDir, "haproxy.cfg"))
		if err != nil {
			suite.T().Error(err.Error())
		}

		// Debug: Print the actual config to see what's generated
		suite.T().Logf("Generated HAProxy config:\n%s", string(contents))

		// Check that -m end is used with wildcard hosts in route-acl
		if !strings.Contains(string(contents), "var(txn.host) -m end .example.local") {
			suite.T().Error("Expected to find 'var(txn.host) -m end .example.local' in HAProxy config")
		}

		// Check that the buggy -m str pattern is NOT used
		if strings.Contains(string(contents), "var(txn.host) -m str *.example.local") {
			suite.T().Error("Found buggy pattern 'var(txn.host) -m str *.example.local' in HAProxy config")
		}

		// Check that route-acl annotation is applied
		if !strings.Contains(string(contents), "path_reg path-in-bug-repro$") {
			suite.T().Error("Expected to find route-acl pattern 'path_reg path-in-bug-repro$' in HAProxy config")
		}
	})
}
