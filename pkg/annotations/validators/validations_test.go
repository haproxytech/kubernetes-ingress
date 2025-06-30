package validators_test

import (
	"testing"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/validators"
)

func TestValidator_Init(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		section string
		wantErr bool
	}{
		{
			name:    "Valid duration",
			field:   "timeout-server",
			value:   "3s",
			section: "backend",
			wantErr: false,
		},
		{
			name:    "Valid duration",
			field:   "timeout-server",
			value:   "3s",
			section: "does-not-exist",
			wantErr: true,
		},
		{
			name:    "Invalid duration",
			field:   "timeout-server",
			value:   "60m",
			section: "backend",
			wantErr: true,
		},
		{
			name:    "Valid duration with zero value",
			field:   "timeout-server",
			value:   "0s",
			section: "backend",
			wantErr: true,
		},
		{
			name:    "Valid int",
			field:   "max-connections",
			value:   "50",
			section: "backend",
			wantErr: false,
		},
		{
			name:    "Invalid int (zero)",
			field:   "max-connections",
			value:   "0",
			section: "backend",
			wantErr: true,
		},
		{
			name:    "Invalid int (too high)",
			field:   "max-connections",
			value:   "1_000_001",
			section: "backend",
			wantErr: true,
		},
		{
			name:    "Invalid int format",
			field:   "max-connections",
			value:   "not-an-int",
			section: "backend",
			wantErr: true,
		},
		{
			name:    "Non-existent rule",
			field:   "non-existent-rule",
			value:   "some-value",
			section: "backend",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := validators.Get()
			v.SetPrefix("haproxy.org")
			if err != nil {
				t.Errorf("Validator.Init() error = %v", err)
				return
			}
			if err := v.Init(example); err != nil {
				t.Errorf("Validator.Init() error = %v", err)
			}
			err = v.ValidateInput(tt.field, tt.value, tt.section)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validator.ValidateInput() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
