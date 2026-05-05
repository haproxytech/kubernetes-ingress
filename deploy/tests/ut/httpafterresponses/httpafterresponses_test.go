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

package httpafterresponses

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/haproxytech/client-native/v6/models"
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/pkg/controller/constants"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (suite *HTTPAfterResponsesSuite) TestHTTPAfterResponses() {
	eventChan := suite.UseHTTPAfterResponsesFixture()
	cfgPath := filepath.Join(suite.test.TempDir, "haproxy.cfg")

	contents, err := os.ReadFile(cfgPath)
	if err != nil {
		suite.T().Fatal(err.Error())
	}

	suite.Run("http-after-response set-header X-After-Test", func() {
		c := strings.Count(string(contents), "http-after-response set-header X-After-Test after-value")
		suite.Exactly(1, c, "expected directive once, got %d", c)
	})

	suite.Run("http-after-response set-status 503 with cond", func() {
		c := strings.Count(string(contents), "http-after-response set-status 503 reason Maintenance if { var(txn.maint) -m bool }")
		suite.Exactly(1, c, "expected directive once, got %d", c)
	})

	// Reload-on-rule-list-change check (the bug-2 part of ebc61be8 applied to
	// HTTPAfterResponseRuleList): submit a Backend CR that changes ONLY the
	// rule list, with no BackendBase change, and confirm the new rule lands
	// in the rendered config.
	suite.Run("rule-list-only change still triggers config update", func() {
		updated := v3.Backend{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backend1cr",
				Namespace: "ns1",
			},
			Spec: v3.BackendSpec{
				Backend: models.Backend{
					BackendBase: models.BackendBase{
						From: constants.DefaultsSectionName,
						Name: "backend1",
					},
					HTTPAfterResponseRuleList: models.HTTPAfterResponseRules{
						{
							Type:      "set-header",
							HdrName:   "X-After-Updated",
							HdrFormat: "updated-value",
						},
					},
				},
			},
		}
		eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.CR_BACKEND, Namespace: updated.Namespace, Name: updated.Name, Data: &updated}
		done := make(chan struct{})
		eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND}
		eventChan <- k8ssync.SyncDataEvent{EventProcessed: done}
		<-done

		updatedContents, errRead := os.ReadFile(cfgPath)
		if errRead != nil {
			suite.T().Fatal(errRead.Error())
		}

		newCount := strings.Count(string(updatedContents), "http-after-response set-header X-After-Updated updated-value")
		suite.Exactly(1, newCount, "expected new directive once after rule-list-only update, got %d", newCount)

		oldCount := strings.Count(string(updatedContents), "http-after-response set-header X-After-Test after-value")
		suite.Exactly(0, oldCount, "expected old directive removed after rule-list-only update, got %d", oldCount)
	})
}
