package validators_test

import (
	"testing"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/validators"
	"github.com/stretchr/testify/require"
)

func TestValidator_Clear(t *testing.T) {
	v, err := validators.Get()
	require.NoError(t, err)

	config := validators.Config{ValidationRules: map[string]validators.Rule{
		"maxconn": {Type: "int", Rule: "value > 0", Section: "backend"},
	}}
	require.NoError(t, v.Set("example.com", config))
	require.NoError(t, v.ValidateInput("maxconn", "10", "backend"))
	require.Contains(t, v.Prefixes(), "example.com/")

	// Deleting the rules must stop granting the annotations they allowed.
	v.Clear()
	require.Error(t, v.ValidateInput("maxconn", "10", "backend"))
	require.Empty(t, v.Prefix())
	require.NotContains(t, v.Prefixes(), "example.com/")
}
