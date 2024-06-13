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

package acls

import (
	"os"
	"path/filepath"
	"strings"
)

func (suite *ACLSuite) TestACL() {
	suite.UseACLFixture()
	contents, err := os.ReadFile(filepath.Join(suite.test.TempDir, "haproxy.cfg"))
	if err != nil {
		suite.T().Error(err.Error())
	}

	suite.Run("acl cookie_found", func() {
		c := strings.Count(string(contents), "acl cookie_found cook(JSESSIONID) -m found")
		suite.Exactly(c, 1, "acl cookie_found is repeated %d times but expected 1", c)
		c = strings.Count(string(contents), "acl is_ticket path_beg -i /ticket")
		suite.Exactly(c, 1, "acl is_ticket is repeated %d times but expected 1", c)
	})

	suite.Run("acl is_ticket", func() {
		c := strings.Count(string(contents), "acl is_ticket path_beg -i /ticket")
		suite.Exactly(c, 1, "acl is_ticket is repeated %d times but expected 1", c)
	})
}
