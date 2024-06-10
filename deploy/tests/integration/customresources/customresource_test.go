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
)

func (suite *CustomResourceSuite) TestGlobalCR() {
	name := suite.T().Name()
	testController := suite.TestControllers[name]

	suite.StartController()
	suite.GlobalCRFixture()

	globals := testController.Store.GetNamespace(suite.globalCREvt.Namespace).CRs.Global
	logtrargets := testController.Store.GetNamespace(suite.globalCREvt.Namespace).CRs.LogTargets
	t := suite.T()

	suite.Run("Adding a global CR creates global and logTargets objects", func() {
		eventProcessedChan := make(chan struct{})
		suite.globalCREvt.EventProcessed = eventProcessedChan
		testController.EventChan <- suite.globalCREvt
		<-eventProcessedChan
		assert.Len(t, globals, 1, "Should find one and only one Global CR")
		assert.Len(t, logtrargets, 1, "Should find one and only one LogTargets CR")
		assert.Containsf(t, globals, suite.globalCREvt.Name, "Global CR of name '%s' not found", suite.globalCREvt.Name)
	})

	suite.Run("Deleting a global CR removes global and logTargets objects", func() {
		suite.globalCREvt.Data = nil
		eventProcessedChan := make(chan struct{})
		suite.globalCREvt.EventProcessed = eventProcessedChan
		testController.EventChan <- suite.globalCREvt
		<-eventProcessedChan
		assert.Empty(t, globals, "No Global CR should be present")
		assert.Empty(t, logtrargets, "No LogTargets should be present")
	})

	suite.StopController()
}
