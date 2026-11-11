package ingress

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func TestUnquotedAnnotationTokens(t *testing.T) {
	tests := []struct {
		name    string
		valid   string
		payload string
		process func(*rules.List, map[string]string) error
	}{
		{
			name: "set-host", valid: "%[req.hdr(host)]",
			payload: "evil.local if TRUE or { str(a) -m str",
			process: func(list *rules.List, values map[string]string) error {
				return NewReqSetHost("set-host", list).Process(store.K8s{}, values)
			},
		},
		{
			name: "request-redirect", valid: "https://example.com:8443",
			payload: "evil.local if TRUE or { str(a) -m str a",
			process: func(list *rules.List, values map[string]string) error {
				return NewHostRedirect(list).NewAnnotation("request-redirect").Process(store.K8s{}, values)
			},
		},
		{
			name: "src-ip-header", valid: " X-Client-IP ",
			payload: "X-Evil) if TRUE or { str(a) -m str a",
			process: func(list *rules.List, values map[string]string) error {
				return NewSrcIPHdr("src-ip-header", list).Process(store.K8s{}, values)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var list rules.List
			require.NoError(t, tt.process(&list, map[string]string{tt.name: tt.valid}))
			require.Len(t, list, 1)
			for _, value := range []string{tt.payload, "x{y}", `x"y`, "x'y", `x\`, "   "} {
				list = nil
				require.Error(t, tt.process(&list, map[string]string{tt.name: value}), value)
				require.Empty(t, list)
			}
			list = nil
			require.NoError(t, tt.process(&list, map[string]string{tt.name: "evil\tif\tTRUE"}))
			require.Empty(t, list)
		})
	}
}

func TestCORSOriginKeepsScope(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "wildcard", value: "*"},
		{name: "origin", value: "https://example.com"},
		{name: "documented regex", value: `^https://(.+\.)?(example-1\.com|example-2\.com)(:\d{1,5})?$`},
		{name: "scope escape", value: "x } || { str(a) -m str a", wantErr: true},
		{name: "closing brace", value: "}", wantErr: true},
		{name: "opening brace", value: "{", wantErr: true},
		{name: "quote", value: `x"y`, wantErr: true},
		{name: "single quote", value: "x'y", wantErr: true},
		{name: "trailing escape", value: `x\`, wantErr: true},
		{name: "odd trailing escapes", value: `x\\\`, wantErr: true},
		{name: "invalid hex escape", value: `x\xZZ`, wantErr: true},
		{name: "truncated hex escape", value: `x\x`, wantErr: true},
		{name: "encoded closing brace", value: `\x7d`, wantErr: true},
		{name: "encoded opening brace", value: `\x7B`, wantErr: true},
		{name: "escaped quote", value: `x\"`, wantErr: true},
		{name: "unique id flag", value: "-u", wantErr: true},
		{name: "pattern file flag", value: "-f", wantErr: true},
		{name: "hex escape", value: `^https://example\x2ecom$`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var list rules.List
			cors := NewResSetCORS(&list)
			values := map[string]string{"cors-enable": "true", "cors-allow-origin": tt.value}
			require.NoError(t, cors.NewAnnotation("cors-enable").Process(store.K8s{}, values))
			err := cors.NewAnnotation("cors-allow-origin").Process(store.K8s{}, values)
			if tt.wantErr {
				require.Error(t, err)
				require.Len(t, list, 1)
				return
			}
			require.NoError(t, err)
			require.Len(t, list, 2)
			header, ok := list[1].(*rules.SetHdr)
			require.True(t, ok)
			require.Equal(t, "if", header.Cond)
			require.NotContains(t, header.CondTest, " || ")
		})
	}
}

func TestCORSHeaderNames(t *testing.T) {
	for _, value := range []string{"*", "X-Foo, Authorization", `X-Foo"`, `X-Foo\`, "X-${SECRET}", "X'Foo"} {
		t.Run(value, func(t *testing.T) {
			var list rules.List
			cors := NewResSetCORS(&list)
			values := map[string]string{"cors-enable": "true", "cors-allow-headers": value}
			require.NoError(t, cors.NewAnnotation("cors-enable").Process(store.K8s{}, values))
			err := cors.NewAnnotation("cors-allow-headers").Process(store.K8s{}, values)
			if value == "*" || value == "X-Foo, Authorization" {
				require.NoError(t, err)
				require.Len(t, list, 2)
				return
			}
			require.Error(t, err)
			require.Len(t, list, 1)
		})
	}
}
