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

package route

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func TestValidateRouteACL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "documented example", value: "cookie(staging) -m found"},
		{name: "path", value: "path_beg /api"},
		{name: "header", value: "hdr(X-Foo) -i bar"},
		{name: "closing brace", value: "path_beg /x } or { path_beg /", wantErr: true},
		{name: "wrapped braces", value: "{ path_beg /x }", wantErr: true},
		{name: "quotes", value: `hdr(X-Foo) -i "bar"`, wantErr: true},
		{name: "comment char", value: "path_beg /x #", wantErr: true},
		{name: "control char", value: "path_beg /x\t", wantErr: true},
		{name: "regex escape", value: `path_reg ^/api/v[0-9]+\.json$`},
		{name: "flag before pattern", value: "path -u 1 /x"},
		{name: "encoded closing brace", value: `path_beg /x \x7d || \x7b always_true`, wantErr: true},
		{name: "encoded uppercase brace", value: `path_beg /x \x7D || \x7B always_true`, wantErr: true},
		{name: "trailing backslash", value: `path_beg /x\`, wantErr: true},
		{name: "odd trailing backslashes", value: `path_beg /x\\\`, wantErr: true},
		{name: "invalid hex escape", value: `path_beg /x\xZZ`, wantErr: true},
		{name: "single quote", value: "path_beg 'x", wantErr: true},
		{name: "trailing unique id flag", value: "path -m found -u", wantErr: true},
		{name: "trailing pattern file flag", value: "path -f", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRouteACL(tt.value)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRoutePathKeepsScope(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "prefix", path: "/foo"},
		{name: "root", path: "/"},
		{name: "space", path: "/a victim_svc_http.", wantErr: true},
		{name: "tab", path: "/a\tvictim_svc_http.", wantErr: true},
		{name: "newline", path: "/a\nvictim.com/ victim_svc_http", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := Route{
				Host:        "example.com",
				Path:        &store.IngressPath{Path: tt.path, PathTypeMatch: store.PATH_TYPE_IMPLEMENTATION_SPECIFIC},
				BackendName: "ns_svc_http",
			}
			_, err := MapRows(route)
			if tt.wantErr {
				require.Error(t, err)
				require.Error(t, AddCustomRoute(route, "path_beg /x", nil))
				return
			}
			require.NoError(t, err)
		})
	}
	for _, path := range []string{`/a\`, `/a\xZZ`, `/a"b`, "/a'b", `/a($|/) } || { always_true } { path -m reg ^/a`} {
		route := Route{Path: &store.IngressPath{Path: path}, BackendName: "ns_svc_http"}
		require.Error(t, AddCustomRoute(route, "path_beg /x", nil), path)
	}
}

func TestRoutePathRefusedRegex(t *testing.T) {
	// The fake HAProxy refuses every rule except the startup probe.
	binary := filepath.Join(t.TempDir(), "haproxy")
	script := "#!/bin/sh\nif grep -q probe; then exit 0; fi\nexit 1\n"
	require.NoError(t, os.WriteFile(binary, []byte(script), 0o700)) //nolint:gosec
	t.Cleanup(func() { common.SetRegexBinary("") })
	common.SetRegexBinary(binary)

	route := Route{
		Host:        "example.com",
		Path:        &store.IngressPath{Path: "/a(", PathTypeMatch: store.PATH_TYPE_PREFIX},
		BackendName: "ns_svc_http",
	}
	require.ErrorContains(t, AddCustomRoute(route, "path_beg /x", nil), "regex")
}
