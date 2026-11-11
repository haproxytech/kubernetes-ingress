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

package ingress

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// The once-keys are process-global and tests reuse the process: fresh names per invocation.
var accessControlSeq atomic.Int64

func TestAccessControlDeprecationNotices(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	// The current name gets the generic removal notice, once.
	denyListName := fmt.Sprintf("deny-list-%d", accessControlSeq.Add(1))
	ann := NewDenyList(denyListName, &rules.List{}, newRecordingMaps())
	require.NoError(t, ann.Process(store.K8s{}, map[string]string{denyListName: "192.168.1.1"}))
	require.Contains(t, buf.String(), fmt.Sprintf("DEPRECATED: annotation %q will be removed", denyListName))
	require.Equal(t, 1, strings.Count(buf.String(), "DEPRECATED"), "warning must be logged once per app run")

	// The deprecated alias warns about the rename only: its fallback must not
	// also mark the new name as used and double-notice the same user.
	buf.Reset()
	aliasName := fmt.Sprintf("deny-list-%d", accessControlSeq.Add(1))
	rulesList := &rules.List{}
	ann = NewDenyList(aliasName, rulesList, newRecordingMaps())
	require.NoError(t, ann.Process(store.K8s{}, map[string]string{"blacklist": "192.168.1.1"}))
	require.Len(t, *rulesList, 1, "the deprecated alias must keep working")
	require.NotContains(t, buf.String(), fmt.Sprintf("annotation %q will be removed", aliasName))
}

// recordingMaps implements maps.Maps and records appended rows so tests can
// assert exactly which addresses end up in the access-control map file.
type recordingMaps struct {
	entries map[maps.Name][]string
}

func newRecordingMaps() *recordingMaps {
	return &recordingMaps{entries: make(map[maps.Name][]string)}
}

func (m *recordingMaps) MapAppend(name maps.Name, row string) {
	if row == "" {
		return
	}
	m.entries[name] = append(m.entries[name], row)
}

func (m *recordingMaps) MapExists(name maps.Name) bool {
	return len(m.entries[name]) != 0
}

func (m *recordingMaps) RefreshMaps(api.HAProxyClient) {}

func (m *recordingMaps) CleanMaps() {}

// allEntries returns the rows of all recorded maps. Process writes to a
// single hash-named map, so this is the content of that map.
func (m *recordingMaps) allEntries() []string {
	var rows []string
	for _, e := range m.entries {
		rows = append(rows, e...)
	}
	return rows
}

// TestAccessControl_Process tests the allow-list/deny-list annotation processing.
// It validates that:
//   - Single IP addresses and CIDR ranges are correctly parsed and stored in a map file
//   - Multiple comma-separated values are supported
//   - Empty elements produced by a trailing comma, consecutive commas or
//     surrounding whitespace are skipped instead of invalidating the whole list
//   - A list without any valid element is still rejected (an empty allow-list
//     map would deny all traffic)
//   - Invalid addresses are rejected with an appropriate error message
//   - Pattern file references (using "patterns/" prefix) are handled correctly
//
//revive:disable-next-line:function-length
func TestAccessControl_Process(t *testing.T) {
	tests := []struct {
		name        string
		annotation  string
		value       string
		denyList    bool
		wantErr     bool
		wantEntries []string
		wantMapPath string
	}{
		{
			name:        "single IP",
			annotation:  "allow-list",
			value:       "192.168.1.1",
			wantEntries: []string{"192.168.1.1"},
		},
		{
			name:        "multiple IPs and CIDRs",
			annotation:  "allow-list",
			value:       "192.168.1.1, 10.0.0.0/8, 172.16.0.0/12",
			wantEntries: []string{"192.168.1.1", "10.0.0.0/8", "172.16.0.0/12"},
		},
		{
			name:        "trailing comma",
			annotation:  "allow-list",
			value:       "192.168.1.0/24,",
			wantEntries: []string{"192.168.1.0/24"},
		},
		{
			name:        "consecutive commas",
			annotation:  "allow-list",
			value:       "192.168.1.1,,10.0.0.0/8",
			wantEntries: []string{"192.168.1.1", "10.0.0.0/8"},
		},
		{
			name:        "leading comma and spaces around elements",
			annotation:  "allow-list",
			value:       " , 192.168.1.1 ,  10.0.0.0/8  ",
			wantEntries: []string{"192.168.1.1", "10.0.0.0/8"},
		},
		{
			name:        "deprecated whitelist alias with trailing comma",
			annotation:  "whitelist",
			value:       "10.0.0.0/16,",
			wantEntries: []string{"10.0.0.0/16"},
		},
		{
			name:        "deny-list with trailing comma",
			annotation:  "deny-list",
			value:       "192.168.1.1,",
			denyList:    true,
			wantEntries: []string{"192.168.1.1"},
		},
		{
			name:       "only commas",
			annotation: "allow-list",
			value:      ",,",
			wantErr:    true,
		},
		{
			name:       "only spaces",
			annotation: "allow-list",
			value:      "   ",
			wantErr:    true,
		},
		{
			name:       "empty value",
			annotation: "allow-list",
			value:      "",
		},
		{
			name:       "invalid address",
			annotation: "allow-list",
			value:      "192.168.1.1, invalid",
			wantErr:    true,
		},
		{
			name:       "invalid CIDR",
			annotation: "allow-list",
			value:      "192.168.1.0/33",
			wantErr:    true,
		},
		{
			name:        "patterns file",
			annotation:  "allow-list",
			value:       "patterns/allowlist",
			wantMapPath: "patterns/allowlist",
		},
		{
			name:       "patterns traversal",
			annotation: "allow-list",
			value:      "patterns/../etc/passwd",
			wantErr:    true,
		},
		{
			name:       "patterns undeclared file",
			annotation: "allow-list",
			value:      "patterns/missing",
			wantErr:    true,
		},
		{
			name:        "pattern reference whitespace",
			annotation:  "allow-list",
			value:       " patterns/allowlist ",
			wantMapPath: "patterns/allowlist",
		},
		{
			name:       "pattern with non-IP contents",
			annotation: "allow-list",
			value:      "patterns/names",
			wantErr:    true,
		},
		{
			name:       "deny-list with non-IP contents",
			annotation: "deny-list",
			value:      "patterns/names",
			denyList:   true,
			wantErr:    true,
		},
		{
			name:       "deprecated name with invalid pattern",
			annotation: "whitelist",
			value:      "patterns/missing",
			wantErr:    true,
		},
		{
			name:       "comment marker cannot remove protection",
			annotation: "allow-list",
			value:      "patterns/allowlist#",
			wantErr:    true,
		},
		{
			name:       "newline cannot remove protection",
			annotation: "allow-list",
			value:      "patterns/allowlist\n",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMaps := newRecordingMaps()
			rulesList := &rules.List{}

			var accessControl *AccessControl
			if tt.denyList {
				accessControl = NewDenyList(tt.annotation, rulesList, mockMaps)
			} else {
				accessControl = NewAllowList(tt.annotation, rulesList, mockMaps)
			}

			k := testPatternFilesStore("allowlist")
			k.ConfigMaps.PatternFiles.Annotations["names"] = "example.invalid\n"
			err := accessControl.Process(k, map[string]string{tt.annotation: tt.value})

			if tt.wantErr {
				assert.ErrorContains(t, err, "denying access")
				require.Len(t, *rulesList, 1)
				assert.Equal(t, &rules.ReqDeny{}, (*rulesList)[0])
				assert.Empty(t, mockMaps.allEntries())
				return
			}

			require.NoError(t, err)

			if tt.value == "" {
				assert.Empty(t, *rulesList)
				return
			}

			require.Len(t, *rulesList, 1)
			reqDeny, ok := (*rulesList)[0].(*rules.ReqDeny)
			require.True(t, ok)
			assert.Equal(t, !tt.denyList, reqDeny.AllowList)

			if tt.wantMapPath != "" {
				assert.Equal(t, maps.Path(tt.wantMapPath), reqDeny.SrcIPsMap)
				return
			}

			assert.ElementsMatch(t, tt.wantEntries, mockMaps.allEntries())
		})
	}
}

func TestAccessControlRejectsSharedInvalidList(t *testing.T) {
	m := newRecordingMaps()
	for range 2 {
		list := &rules.List{}
		ann := NewAllowList("allow-list", list, m)
		err := ann.Process(testPatternFilesStore(), map[string]string{"allow-list": "10.0.0.1,invalid"})
		require.Error(t, err)
		require.Equal(t, rules.List{&rules.ReqDeny{}}, *list)
		require.Empty(t, m.allEntries())
	}
}

func TestAccessControlPatternUpdate(t *testing.T) {
	k := testPatternFilesStore("ips")
	annotations := map[string]string{"allow-list": "patterns/ips"}
	for _, content := range []string{"10.0.0.1\n", "not-an-ip\n", "::1\n"} {
		k.ConfigMaps.PatternFiles.Annotations["ips"] = content
		list := &rules.List{}
		ann := NewAllowList("allow-list", list, newRecordingMaps())
		err := ann.Process(k, annotations)
		require.Len(t, *list, 1)
		if content == "not-an-ip\n" {
			require.Error(t, err)
			assert.Equal(t, &rules.ReqDeny{}, (*list)[0])
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, &rules.ReqDeny{SrcIPsMap: "patterns/ips", AllowList: true}, (*list)[0])
	}
}

func TestAccessControlValuePrecedence(t *testing.T) {
	tests := []struct {
		name        string
		annotations []map[string]string
		want        string
	}{
		{name: "absent"},
		{
			name:        "new name beats deprecated name",
			annotations: []map[string]string{{"whitelist": "10.0.0.1"}, {"allow-list": "10.0.0.2"}},
			want:        "10.0.0.2",
		},
		{
			name:        "first map wins",
			annotations: []map[string]string{{"allow-list": "10.0.0.1"}, {"allow-list": "10.0.0.2"}},
			want:        "10.0.0.1",
		},
		{
			name:        "empty value masks inherited value",
			annotations: []map[string]string{{"allow-list": ""}, {"allow-list": "10.0.0.2"}},
		},
		{
			name:        "empty new name falls back to deprecated name",
			annotations: []map[string]string{{"allow-list": "", "whitelist": "10.0.0.1"}},
			want:        "10.0.0.1",
		},
		{
			name:        "invalid new name cannot fall back to deprecated name",
			annotations: []map[string]string{{"allow-list": "bad#value", "whitelist": "10.0.0.1"}},
			want:        "bad#value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ann := NewAllowList("allow-list", &rules.List{}, newRecordingMaps())
			assert.Equal(t, tt.want, ann.GetAnnotationValue(tt.annotations...))
		})
	}
}
