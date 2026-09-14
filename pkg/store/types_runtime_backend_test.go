package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeServerNameIsStableAndShort(t *testing.T) {
	a := RuntimeEndpoint{Address: "10.0.0.1", Port: 8080}.ComputeServerName()
	b := RuntimeEndpoint{Address: "10.0.0.1", Port: 8081}.ComputeServerName()
	v6 := RuntimeEndpoint{Address: "fd00::8", Port: 8080}.ComputeServerName()

	require.Equal(t, a, RuntimeEndpoint{Address: "10.0.0.1", Port: 8080}.ComputeServerName())
	require.NotEqual(t, a, b)
	require.NotEqual(t, a, v6)
	require.Len(t, a, 1+serverNameHashLen)
	require.Regexp(t, `^s[0-9a-f]+$`, a)
}
