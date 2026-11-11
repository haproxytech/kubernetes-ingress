package ingress

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func TestReqCaptureExpressionScope(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "headers and cookies", input: "hdr(Host)\ncookie(my-cookie)\nreq.fhdr(user-agent)"},
		{name: "fetch without arguments", input: "ssl_fc_cipher"},
		{name: "converters", input: "hdr(Host),lower,regsub(foo,bar,g)"},
		{name: "separate converters", input: "hdr(Host) lower  upper"},
		{name: "regex escape", input: `hdr(Host),regsub(\.,_,g)`},
		{name: "condition words in arguments", input: `str(if),regsub(unless,if,g)`},
		{name: "quoted space", input: `str("hello world")`, wantErr: true},
		{name: "escaped space", input: `str(hello\ world)`, wantErr: true},
		{name: "single quotes", input: `'str(hello)'`, wantErr: true},
		{name: "escaped quote", input: `hdr(Host),regsub(\"(foo|bar)\",blah,g)`, wantErr: true},
		{name: "empty quoted word", input: `hdr(x) "" lower`, wantErr: true},
		{name: "invalid hex escape", input: `str(\xZZ)`, wantErr: true},
		{name: "empty lines", input: "\nhdr(Host)\n\ncookie(name)\n"},
		{name: "injected scope", input: "hdr(x) len 1 if TRUE or { str(a) -m str", wantErr: true},
		{name: "injected length", input: "hdr(x) len 1", wantErr: true},
		{name: "injected slot", input: "hdr(x) id 0", wantErr: true},
		{name: "injected if", input: "hdr(x) if TRUE", wantErr: true},
		{name: "injected unless", input: "hdr(x) unless TRUE", wantErr: true},
		{name: "quoted directive", input: `hdr(x) "len" 1`, wantErr: true},
		{name: "encoded directive", input: `hdr(x) \x6cen 1`, wantErr: true},
		{name: "tab", input: "hdr(x)\tlen 1", wantErr: true},
		{name: "quoted tab", input: "str(\"a\tb\")", wantErr: true},
		{name: "brace in argument", input: `str({x})`},
		{name: "unclosed quote", input: `str("x)`, wantErr: true},
		{name: "trailing escape", input: `hdr(x)\`, wantErr: true},
		{name: "environment expansion", input: `str("$VALUE")`, wantErr: true},
		{name: "malformed expression", input: `hdr(x)garbage`, wantErr: true},
		{name: "no partial publication", input: "hdr(Host)\nhdr(x) len 1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var list rules.List
			capture := NewReqCapture(&list)
			err := capture.NewAnnotation("request-capture").Process(store.K8s{}, map[string]string{"request-capture": tt.input})
			if tt.wantErr {
				require.Error(t, err)
				require.Empty(t, list)
				require.Empty(t, capture.capture)
				return
			}
			require.NoError(t, err)
			var samples []string
			for _, sample := range strings.Split(tt.input, "\n") {
				if sample != "" {
					samples = append(samples, sample)
				}
			}
			require.Len(t, list, len(samples))
			require.Len(t, capture.capture, len(samples))
			for i, sample := range samples {
				require.Equal(t, sample, list[i].(*rules.ReqCapture).Expression)
			}
			err = capture.NewAnnotation("request-capture-len").Process(store.K8s{}, map[string]string{"request-capture-len": "350"})
			require.NoError(t, err)
			for _, rule := range capture.capture {
				require.EqualValues(t, 350, rule.CaptureLen)
			}
		})
	}
}
