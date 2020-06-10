package haproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer_UpdateSendProxy(t *testing.T) {
	type tc struct {
		enum      string
		expected  Server
		riseError bool
	}
	tt := map[string]tc{
		"proxy":    {enum: "proxy", expected: Server{SendProxy: "enabled"}, riseError: false},
		"proxy-v1": {enum: "proxy-v1", expected: Server{SendProxy: "enabled"}, riseError: false},
		"proxy-v2": {enum: "proxy-v2", expected: Server{SendProxyV2: "enabled"}, riseError: false},
		"error":    {enum: "error", expected: Server{}, riseError: true},
	}
	for name, c := range tt {
		t.Run(name, func(t *testing.T) {
			s := Server{}
			err := s.UpdateSendProxy(c.enum)
			if !c.riseError {
				assert.Nil(t, err)
			}
			if c.riseError {
				assert.Error(t, err)
			}
			assert.Equal(t, c.expected, s)
		})
	}
}

func TestServer_ResetSendProxy(t *testing.T) {
	tt := map[string]Server{
		"enabled-proxy-v1":  {SendProxy: "enabled"},
		"enabled-proxy-v2":  {SendProxyV2: "enabled"},
		"disabled-proxy-v1": {SendProxy: "disabled"},
		"disabled-proxy-v2": {SendProxyV2: "disabled"},
	}
	for name, c := range tt {
		t.Run(name, func(t *testing.T) {
			c.ResetSendProxy()
			assert.Equal(t, Server{}, c)
		})
	}

}
