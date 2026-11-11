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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// testPatternFilesStore returns a store whose pattern files ConfigMap declares the given files.
func testPatternFilesStore(files ...string) store.K8s {
	annotations := make(map[string]string, len(files))
	for _, file := range files {
		annotations[file] = "10.0.0.1\n"
	}
	return store.K8s{ConfigMaps: store.ConfigMaps{PatternFiles: &store.ConfigMap{
		Namespace:   "default",
		Name:        "pattern-files",
		Annotations: annotations,
	}}}
}

func TestPatternPath(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		store      store.K8s
		wantErr    bool
		wantResult maps.Path
	}{
		{
			name:       "declared file",
			value:      "patterns/ips",
			store:      testPatternFilesStore("ips"),
			wantResult: "patterns/ips",
		},
		{
			name:       "several declared files",
			value:      "patterns/names",
			store:      testPatternFilesStore("ips", "names"),
			wantResult: "patterns/names",
		},
		{
			name:    "traversal",
			value:   "patterns/../etc/passwd",
			store:   testPatternFilesStore("ips"),
			wantErr: true,
		},
		{
			name:    "nested path",
			value:   "patterns/sub/ips",
			store:   testPatternFilesStore("ips"),
			wantErr: true,
		},
		{
			name:    "empty file name",
			value:   "patterns/",
			store:   testPatternFilesStore("ips"),
			wantErr: true,
		},
		{
			name:    "undeclared file",
			value:   "patterns/missing",
			store:   testPatternFilesStore("ips"),
			wantErr: true,
		},
		{
			name:    "no pattern files configmap",
			value:   "patterns/ips",
			store:   store.K8s{},
			wantErr: true,
		},
		{
			name:    "space splits the token",
			value:   "patterns/ips names",
			store:   testPatternFilesStore("ips"),
			wantErr: true,
		},
		{
			name:    "quote",
			value:   `patterns/ips"`,
			store:   testPatternFilesStore("ips"),
			wantErr: true,
		},
		{
			name:    "backslash",
			value:   `patterns/ips\`,
			store:   testPatternFilesStore("ips"),
			wantErr: true,
		},
		{
			name:    "comment marker",
			value:   "patterns/ips#",
			store:   testPatternFilesStore("ips"),
			wantErr: true,
		},
		{
			name:    "dot cannot name a file",
			value:   "patterns/.",
			store:   testPatternFilesStore("."),
			wantErr: true,
		},
		{
			name:    "dot-dot cannot name a file",
			value:   "patterns/..",
			store:   testPatternFilesStore(".."),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := patternPath(tt.store, "allow-list", tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, path)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantResult, path)
		})
	}
}

func TestPatternPathContents(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "IPv4 and IPv6", content: "10.0.0.1\n::1\n"},
		{name: "CIDRs", content: "10.0.0.0/8\n2001:db8::/32\n"},
		{name: "IPv4 netmask", content: "10.0.0.0/255.0.0.0\n"},
		{name: "non-contiguous IPv4 mask", content: "10.0.0.0/255.0.255.0\n"},
		{name: "comments and blank lines", content: "# comment\n\n \t\n10.0.0.1\n"},
		{name: "leading whitespace", content: " \t10.0.0.1\n"},
		{name: "CRLF", content: "10.0.0.1\r\n"},
		{name: "empty file"},
		{name: "comment-only file", content: "# comment\n"},
		{name: "names", content: "example.invalid\n", wantErr: true},
		{name: "mixed addresses and names", content: "10.0.0.1\nexample.invalid\n", wantErr: true},
		{name: "bad CIDR", content: "10.0.0.1/33\n", wantErr: true},
		{name: "indented comment", content: " # comment\n", wantErr: true},
		{name: "inline comment", content: "10.0.0.1 # comment\n", wantErr: true},
		{name: "trailing whitespace", content: "10.0.0.1 \n", wantErr: true},
		{name: "unicode whitespace", content: "\u00a010.0.0.1\n", wantErr: true},
		{name: "NUL", content: "10.0.0.1\x00ignored\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := testPatternFilesStore("ips")
			k.ConfigMaps.PatternFiles.Annotations["ips"] = tt.content
			path, err := patternPath(k, "allow-list", "patterns/ips")
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, path)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, maps.Path("patterns/ips"), path)
		})
	}
}
