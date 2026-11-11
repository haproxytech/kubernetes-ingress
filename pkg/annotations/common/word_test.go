package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnquotedWord(t *testing.T) {
	tests := []struct {
		value string
		want  string
		ok    bool
	}{
		{value: "/foo", want: "/foo", ok: true},
		{value: `^/foo\.(.*)`, want: `^/foo\.(.*)`, ok: true},
		{value: `/\1`, want: `/\1`, ok: true},
		{value: `\d{1,5}`, want: `\d{1,5}`, ok: true},
		{value: `x\\`, want: `x\`, ok: true},
		{value: `x\$`, want: "x$", ok: true},
		{value: "x$HOME", want: "x$HOME", ok: true},
		{value: `\x2F`, want: "/", ok: true},
		{value: `\x7d`, want: "}", ok: true},
		{value: `\x7B`, want: "{", ok: true},
		{value: ""},
		{value: `x\`},
		{value: `x\\\`},
		{value: `x\xZZ`},
		{value: `x\x`},
		{value: `x\x2`},
		{value: `x\x0a`},
		{value: `x\x7f`},
		{value: `x\n`},
		{value: `x\t`},
		{value: `x\ y`},
		{value: `x\"`},
		{value: `x\'`},
		{value: `x\#`},
		{value: `x"y`},
		{value: "x'y"},
		{value: "x#y"},
		{value: "x y"},
		{value: "x\ty"},
		{value: "x\x7f"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, ok := UnquotedWord(tt.value)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}
