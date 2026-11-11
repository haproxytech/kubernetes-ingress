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
	"log"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func TestSetHdrValues(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() {
		log.SetOutput(previous)
		require.NotContains(t, output.String(), "password")
		require.NotContains(t, output.String(), "SECRET_PASSWORD")
	})

	tests := []struct {
		name       string
		value      string
		wantErr    bool
		wantFormat string
	}{
		{name: "plain value", value: "X-Foo bar", wantFormat: `"bar"`},
		{name: "quoted value", value: `X-Foo "bar baz"`, wantFormat: `"bar baz"`},
		{name: "quote breakout rejected", value: `X-Foo "bar" baz`, wantErr: true},
		{name: "comment in value discarded", value: "X-Foo bar # baz"},
		{name: "comment in quoted value discarded", value: `X-Foo "bar#baz"`},
		{name: "comment in name discarded", value: `X#Foo bar`},
		{name: "carriage return rejected", value: "X-Foo bar\r", wantErr: true},
		{name: "missing header name", value: "password", wantErr: true},
		{name: "trailing backslash", value: `X-Foo password\`, wantErr: true},
		{name: "quoted trailing backslash", value: `X-Foo "password\"`, wantErr: true},
		{name: "malformed hex escape", value: `X-Foo password\xQZ`, wantErr: true},
		{name: "quoted malformed hex escape", value: `X-Foo "password\xQZ"`, wantErr: true},
		{name: "environment reference", value: `X-Foo $SECRET_PASSWORD`, wantErr: true},
		{name: "quoted environment reference", value: `X-Foo "${SECRET_PASSWORD}"`, wantErr: true},
		{name: "empty header name", value: " bar", wantErr: true},
		{name: "colon in header name", value: "X-Foo: bar", wantErr: true},
		{name: "brace in header name", value: "X-Foo} bar", wantErr: true},
		{name: "single quote in header name", value: "X'Foo bar", wantErr: true},
		{name: "environment in header name", value: "X$SECRET bar", wantErr: true},
		{name: "token header name", value: "X_Foo-1 bar", wantFormat: `"bar"`},
		{name: "empty value", value: "X-Foo ", wantErr: true},
		{name: "empty quoted value", value: `X-Foo ""`, wantErr: true},
		{name: "lone quote", value: `X-Foo "`, wantErr: true},
		{name: "quoted spaces", value: `X-Foo "a b"`, wantFormat: `"a b"`},
	}
	for _, response := range []bool{false, true} {
		name := "request-set-header"
		if response {
			name = "response-set-header"
		}
		t.Run(name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					var list rules.List
					ann := NewReqSetHdr(name, &list)
					if response {
						ann = NewResSetHdr(name, &list)
					}
					err := ann.Process(store.K8s{}, map[string]string{name: tt.value})
					if tt.wantErr {
						require.Error(t, err)
						require.NotContains(t, err.Error(), "password")
						require.Empty(t, list)
						return
					}
					require.NoError(t, err)
					if tt.wantFormat == "" {
						require.Empty(t, list)
						return
					}
					require.Len(t, list, 1)
					require.Equal(t, tt.wantFormat, list[0].(*rules.SetHdr).HdrFormat)
					require.Equal(t, response, list[0].(*rules.SetHdr).Response)
				})
			}
		})
	}
}

func TestSetHdrMultilineStillWorks(t *testing.T) {
	var list rules.List
	ann := NewReqSetHdr("request-set-header", &list)
	err := ann.Process(store.K8s{}, map[string]string{"request-set-header": "X-Foo bar\nX-Baz qux"})
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestPathRewriteValues(t *testing.T) {
	var list rules.List
	ann := NewReqPathRewrite("path-rewrite", &list)
	err := ann.Process(store.K8s{}, map[string]string{"path-rewrite": "(.*) /foo#bar"})
	require.NoError(t, err)
	require.Empty(t, list)

	err = ann.Process(store.K8s{}, map[string]string{"path-rewrite": "(.*) /foo\n(.*) /bar"})
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestPathRewriteTokens(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "single path", value: "/"},
		{name: "prefix", value: `(.*) /foo\1`},
		{name: "suffix", value: `([^?]*)(\?(.*))? \1/foo\2`},
		{name: "strip", value: `/foo/(.*) /\1`},
		{name: "escaped backslash", value: `(.*) /x\\`},
		{name: "hex escape", value: `(.*) /x\x2F`},
		{name: "quotes spanning parts", value: `"a b"`, wantErr: true},
		{name: "trailing quote", value: `(.*) x"`, wantErr: true},
		{name: "single quote", value: `(.*) x'`, wantErr: true},
		{name: "escaped quote", value: `(.*) /x\"`, wantErr: true},
		{name: "trailing backslash", value: `(.*) x\`, wantErr: true},
		{name: "odd trailing backslashes", value: `(.*) x\\\`, wantErr: true},
		{name: "invalid hex escape", value: `(.*) /x\xZZ`, wantErr: true},
		{name: "truncated hex escape", value: `(.*) /x\x2`, wantErr: true},
		{name: "second line invalid", value: "(.*) /foo\n(.*) x\\", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var list rules.List
			err := NewReqPathRewrite("path-rewrite", &list).Process(store.K8s{}, map[string]string{"path-rewrite": tt.value})
			if tt.wantErr {
				require.Error(t, err)
				for _, rule := range list {
					rewrite, ok := rule.(*rules.ReqPathRewrite)
					require.True(t, ok)
					require.NotContains(t, rewrite.PathMatch+rewrite.PathFmt, `x\`)
				}
				return
			}
			require.NoError(t, err)
			require.Len(t, list, 1)
		})
	}
}

func TestReqCaptureValues(t *testing.T) {
	var list rules.List
	p := NewReqCapture(&list)
	ann := p.NewAnnotation("request-capture")
	err := ann.Process(store.K8s{}, map[string]string{"request-capture": "hdr(Host) len 128 #"})
	require.NoError(t, err)
	require.Empty(t, list)

	err = ann.Process(store.K8s{}, map[string]string{"request-capture": "hdr(Host)\nhdr(User-Agent)"})
	require.NoError(t, err)
	require.Len(t, list, 2)
}
