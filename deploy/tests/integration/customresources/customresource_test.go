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

package customresources

import (
	"github.com/stretchr/testify/assert"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *CustomResourceSuite) TestGlobalCR() {
	eventChan, s, globalCREvt := suite.GlobalCRFixture()

	globals := s.GetNamespace(globalCREvt.Namespace).CRs.Global
	logtrargets := s.GetNamespace(globalCREvt.Namespace).CRs.LogTargets
	t := suite.T()

	suite.Run("Adding a global CR creates global and logTargets objects", func() {
		eventChan <- globalCREvt
		suite.Require().Eventually(func() bool {
			return assert.Len(t, globals, 1, "Should find one and only one Global CR") ||
				assert.Len(t, logtrargets, 1, "Should find one and only one LogTargets CR") ||
				assert.Containsf(t, globals, globalCREvt.Name, "Global CR of name '%s' not found", globalCREvt.Name)
		}, e2e.WaitDuration, e2e.TickDuration)
	})

	suite.Run("Deleting a global CR removes global and logTargets objects", func() {
		globalCREvt.Data = nil
		eventChan <- globalCREvt
		suite.Require().Eventually(func() bool {
			return assert.Len(t, globals, 0, "No Global CR should be present") ||
				assert.Len(t, logtrargets, 0, "No LogTargets should be present")
		}, e2e.WaitDuration, e2e.TickDuration)
	})

	close(eventChan)
}
