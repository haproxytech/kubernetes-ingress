package handler

import (
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/stretchr/testify/require"
)

func TestTCPFrontendLogFormat(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   string
	}{
		{name: "empty", format: "", want: ""},
		{name: "plain", format: "%ci:%cp [%t] %ft", want: "'%ci:%cp [%t] %ft'"},
		{name: "trimmed", format: "  %ci  ", want: "'%ci'"},
		{name: "double quote and tab", format: "%ci\t\"%b\"", want: "'%ci\t\"%b\"'"},
		{name: "single quote", format: "%ci' maxconn 1 #", want: ""},
		{name: "line feed", format: "%ci\nglobal", want: ""},
		{name: "carriage return", format: "%ci\rglobal", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := models.Frontend{FrontendBase: models.FrontendBase{Name: "fe", LogFormat: tt.format}}
			applyFrontendOverride("ns", &fe)
			require.Equal(t, tt.want, fe.LogFormat)
			require.Equal(t, "tcp", fe.Mode)
		})
	}
}
