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

package configsnippet_test

import (
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
)

func (suite *DisableConfigSnippetSuite) TestUpdateFrontendConfigSnippet() {
	suite.StartController()
	defer suite.StopController()
	suite.setupTest()

	configMapEvent := func(annotations map[string]string) k8ssync.SyncDataEvent {
		return k8ssync.SyncDataEvent{
			SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: newConfigMap(annotations),
		}
	}

	// 2 snippets - frontend: http, https
	suite.disableConfigSnippetFixture(configMapEvent(map[string]string{"frontend-config-snippet": "unique-id-header X-Unique-ID"}))
	suite.ExpectHaproxyConfigContains("unique-id-header X-Unique-ID", 2)

	suite.disableConfigSnippetFixture(configMapEvent(map[string]string{"frontend-config-snippet": "unique-id-header X-Request-ID"}))
	suite.ExpectHaproxyConfigContains("unique-id-header X-Unique-ID", 0)
	suite.ExpectHaproxyConfigContains("unique-id-header X-Request-ID", 2)

	// The snippet is removed while the configmap keeps other settings.
	suite.disableConfigSnippetFixture(configMapEvent(map[string]string{"maxconn": "1000"}))
	suite.ExpectHaproxyConfigContains("unique-id-header", 0)
}
