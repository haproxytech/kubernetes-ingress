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

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func TestSetHdrValues(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		wantErr    bool
		wantFormat string
	}{
		{name: "plain value", value: "X-Foo bar", wantFormat: `"bar"`},
		{name: "comment in value is quoted", value: "X-Foo bar # baz", wantFormat: `"bar # baz"`},
		{name: "quoted value keeps comment literal", value: `X-Foo "bar#baz"`, wantFormat: `"bar#baz"`},
		{name: "quote breakout rejected", value: `X-Foo "bar" #`, wantErr: true},
		{name: "quote inside value rejected", value: `X-Foo bar" #`, wantErr: true},
		{name: "comment in name rejected", value: `X#Foo bar`, wantErr: true},
		{name: "carriage return rejected", value: "X-Foo bar\r", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var list rules.List
			ann := NewReqSetHdr("request-set-header", &list)
			err := ann.Process(store.K8s{}, map[string]string{"request-set-header": tt.value})
			if tt.wantErr {
				require.Error(t, err)
				require.Empty(t, list)
				return
			}
			require.NoError(t, err)
			require.Len(t, list, 1)
			require.Equal(t, tt.wantFormat, list[0].(*rules.SetHdr).HdrFormat)
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
	require.Error(t, err)
	require.Empty(t, list)

	err = ann.Process(store.K8s{}, map[string]string{"path-rewrite": "(.*) /foo\n(.*) /bar"})
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestReqCaptureValues(t *testing.T) {
	var list rules.List
	p := NewReqCapture(&list)
	ann := p.NewAnnotation("request-capture")
	err := ann.Process(store.K8s{}, map[string]string{"request-capture": "hdr(Host) len 128 #"})
	require.Error(t, err)
	require.Empty(t, list)

	err = ann.Process(store.K8s{}, map[string]string{"request-capture": "hdr(Host)\nhdr(User-Agent)"})
	require.NoError(t, err)
	require.Len(t, list, 2)
}
