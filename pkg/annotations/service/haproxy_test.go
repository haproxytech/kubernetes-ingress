package service

import (
	"os/exec"
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/stretchr/testify/assert"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// TestValuesRefusedByHAProxy needs a real HAProxy, as only it knows what it refuses.
func TestValuesRefusedByHAProxy(t *testing.T) {
	binary, err := exec.LookPath("haproxy")
	if err != nil {
		t.Skip("haproxy is not installed")
	}
	t.Cleanup(func() { common.SetHAProxyBinary("") })
	common.SetHAProxyBinary(binary)

	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{"check-http", "/health", true},
		{"check-http", "GET /health HTTP/1.1", true},
		{"check-http", "GET /health HTTP/1.1 extra words", false},
		{"timeout-server", "30s", true},
		{"timeout-server", "-5s", false},
		{"timeout-server", "99999999999s", false},
		{"timeout-check", "2s", true},
		{"timeout-check", "-2s", false},
		{"check-interval", "2s", true},
		{"check-interval", "0", false},
		{"check-interval", "-2s", false},
	}
	for _, tt := range tests {
		t.Run(tt.name+" "+tt.value, func(t *testing.T) {
			backend := &models.Backend{}
			values := map[string]string{tt.name: tt.value}
			var err error
			switch tt.name {
			case "check-http":
				err = NewCheckHTTP(tt.name, backend).Process(store.K8s{}, values)
			case "timeout-server":
				err = NewTimeoutServer(tt.name, backend).Process(store.K8s{}, values)
			case "timeout-check":
				err = NewTimeoutCheck(tt.name, backend).Process(store.K8s{}, values)
			case "check-interval":
				err = NewCheckInter(tt.name, backend).Process(store.K8s{}, values)
			}
			if tt.valid {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
		})
	}
}
