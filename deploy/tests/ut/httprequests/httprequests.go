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

package httprequests

import (
	"os"
	"path/filepath"
	"strings"
)

func (suite *HTTPRequestsSuite) TestHTTPRequests() {
	suite.UseHTTPRequestsFixture()
	contents, err := os.ReadFile(filepath.Join(suite.test.TempDir, "haproxy.cfg"))
	if err != nil {
		suite.T().Fatal(err.Error())
	}

	suite.Run("http-request set-var(txn.admintenant)", func() {
		c := strings.Count(string(contents), "http-request set-var(txn.admintenant) str({{RUN.serviceId2}})")
		suite.Exactly(c, 1, "http-request set-var(txn.admintenant) is repeated %d times but expected 1", c)
	})

	suite.Run("http-request track-sc1 txn.key", func() {
		c := strings.Count(string(contents), " table connected.local if cookie_found")
		suite.Exactly(c, 1, "http-request track-sc1 txn.key is repeated %d times but expected 1", c)
	})
}
