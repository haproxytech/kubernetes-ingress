package api

import (
	"testing"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/stretchr/testify/require"
)

// The runtime backend keeps its name after the config backend was deleted
// (Ingress removed); the next endpoint event must not crash the controller.
func TestSyncNewServersSkipsBackendMissingFromConfig(t *testing.T) {
	client := &clientNative{backends: map[string]Backend{}}
	backend := &store.RuntimeBackend{
		Name:        "ns_svc_http",
		HAProxySrvs: map[string]*store.HAProxySrv{},
	}
	endpoints := store.RuntimeEndpoints{
		{Address: "10.0.0.1", Port: 8080}: {},
	}

	require.NotPanics(t, func() {
		require.NoError(t, client.SyncNewServers(backend, endpoints))
	})
	require.Empty(t, backend.HAProxySrvs)
}

func TestReusableMaintServersExcludesDeleted(t *testing.T) {
	srvs := map[string]*store.HAProxySrv{
		"sA": {Name: "sA", Address: "", Port: 1},                // in MAINT, still in the process
		"sB": {Name: "sB", Address: "", Port: 1, Deleted: true}, // already `del server`-ed
		"sC": {Name: "sC", Address: "10.0.0.3", Port: 8080},     // live
	}

	reusable := reusableMaintServers(srvs)

	require.Len(t, reusable, 1)
	require.Contains(t, reusable, "sA")
}
