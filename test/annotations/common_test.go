package annotations_test

import (
	"testing"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
)

func TestEnsureQuoted(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple string",
			input: "test",
			want:  "\"test\"",
		},
		{
			name:  "already quoted",
			input: "\"test\"",
			want:  "\"test\"",
		},
		{
			name:  "empty string",
			input: "",
			want:  "\"\"",
		},
		{
			name:  "starts with quote",
			input: "\"test",
			want:  "\"test\"",
		},
		{
			name:  "ends with quote",
			input: "test\"",
			want:  "\"test\"",
		},
		{
			name:  "single quote",
			input: "\"",
			want:  "\"\"",
		},
		{
			name:  "empty quoted string",
			input: "\"\"",
			want:  "\"\"",
		},
		{
			name:  "string with internal quotes",
			input: "te\"st",
			want:  "\"te\"st\"",
		},
		{
			name:  "string with leading/trailing spaces",
			input: " test ",
			want:  "\" test \"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := common.EnsureQuoted(tt.input); got != tt.want {
				t.Errorf("EnsureQuoted() = %v, want %v", got, tt.want)
			}
		})
	}
}
