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

package handler

import (
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
)

func TestMergeLists(t *testing.T) {
	live := []string{"live-1", "live-2"}
	crList := []string{"cr-1"}

	tests := []struct {
		name     string
		mode     string
		crList   []string
		expected []string
	}{
		{
			name:     "append is the default",
			mode:     "",
			crList:   crList,
			expected: []string{"live-1", "live-2", "cr-1"},
		},
		{
			name:     "append puts the custom resource entries last",
			mode:     annotations.ListsMergeAppend,
			crList:   crList,
			expected: []string{"live-1", "live-2", "cr-1"},
		},
		{
			name:     "prepend puts the custom resource entries first",
			mode:     annotations.ListsMergePrepend,
			crList:   crList,
			expected: []string{"cr-1", "live-1", "live-2"},
		},
		{
			name:     "override replaces the live list",
			mode:     annotations.ListsMergeOverride,
			crList:   crList,
			expected: []string{"cr-1"},
		},
		{
			name:     "override keeps the live list when the custom resource omits it",
			mode:     annotations.ListsMergeOverride,
			crList:   nil,
			expected: live,
		},
		{
			name:     "append keeps the live list when the custom resource omits it",
			mode:     annotations.ListsMergeAppend,
			crList:   nil,
			expected: live,
		},
		{
			name:     "prepend keeps the live list when the custom resource omits it",
			mode:     annotations.ListsMergePrepend,
			crList:   nil,
			expected: live,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := mergeLists(test.mode, live, test.crList)
			if len(result) != len(test.expected) {
				t.Fatalf("expected %v, got %v", test.expected, result)
			}
			for i := range result {
				if result[i] != test.expected[i] {
					t.Fatalf("expected %v, got %v", test.expected, result)
				}
			}
			// The live list must never be mutated.
			if len(live) != 2 || live[0] != "live-1" || live[1] != "live-2" {
				t.Fatalf("the live list was mutated: %v", live)
			}
		})
	}
}

func TestZeroFrontendLists(t *testing.T) {
	frontend := &models.Frontend{
		ACLList:                   models.Acls{},
		BackendSwitchingRuleList:  models.BackendSwitchingRules{},
		CaptureList:               models.Captures{},
		FilterList:                models.Filters{},
		HTTPAfterResponseRuleList: models.HTTPAfterResponseRules{},
		HTTPErrorRuleList:         models.HTTPErrorRules{},
		HTTPRequestRuleList:       models.HTTPRequestRules{},
		HTTPResponseRuleList:      models.HTTPResponseRules{},
		LogTargetList:             models.LogTargets{},
		QUICInitialRuleList:       models.QUICInitialRules{},
		SSLFrontUses:              models.SSLFrontUses{},
		TCPRequestRuleList:        models.TCPRequestRules{},
	}

	zeroFrontendLists(frontend)

	if frontend.ACLList != nil ||
		frontend.BackendSwitchingRuleList != nil ||
		frontend.CaptureList != nil ||
		frontend.FilterList != nil ||
		frontend.HTTPAfterResponseRuleList != nil ||
		frontend.HTTPErrorRuleList != nil ||
		frontend.HTTPRequestRuleList != nil ||
		frontend.HTTPResponseRuleList != nil ||
		frontend.LogTargetList != nil ||
		frontend.QUICInitialRuleList != nil ||
		frontend.SSLFrontUses != nil ||
		frontend.TCPRequestRuleList != nil {
		t.Fatal("a list field was not emptied")
	}
}
