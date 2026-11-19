package api

import (
	"errors"
	"fmt"
	"testing"

	clientnative "github.com/haproxytech/client-native/v6"
	"github.com/haproxytech/client-native/v6/configuration"
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/client-native/v6/runtime"
	rutracker "github.com/haproxytech/kubernetes-ingress/pkg/runtime-update-tracker"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/stretchr/testify/require"
)

func TestSyncBackendSrvsRetirementAndReturn(t *testing.T) {
	for _, test := range []struct {
		name      string
		deleteErr error
	}{
		{"busy server is reused", errors.New("server still has connections")},
		{"deleted server is recreated", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			tracker := rutracker.GetRuntimeUpdateTracker()
			tracker.Reset()
			t.Cleanup(tracker.Reset)

			a := store.RuntimeEndpoint{Address: "192.0.2.1", Port: 8080}
			b := store.RuntimeEndpoint{Address: "192.0.2.2", Port: 8080}
			retiring := &store.HAProxySrv{Name: a.ComputeServerName(), Address: a.Address, Port: a.Port}
			surviving := &store.HAProxySrv{Name: b.ComputeServerName(), Address: b.Address, Port: b.Port}
			survivorBefore := *surviving
			backend := &store.RuntimeBackend{
				Name:      "be",
				Endpoints: store.RuntimeEndpoints{b: {}},
				HAProxySrvs: map[string]*store.HAProxySrv{
					retiring.Name:  retiring,
					surviving.Name: surviving,
				},
			}
			rt := &retirementRuntime{deleteErr: test.deleteErr}
			client := &clientNative{
				nativeAPI: retirementClient{rt: rt},
				backends: map[string]Backend{
					"be": {Backend: models.Backend{BackendBase: models.BackendBase{Name: "be"}}},
				},
			}
			target := "be/" + retiring.Name

			require.NoError(t, client.SyncBackendSrvs(backend))
			require.Equal(t, []string{
				"set server " + target + " state maint;",
				"del server " + target,
			}, rt.calls)
			require.Empty(t, retiring.Address)
			require.Equal(t, int64(1), retiring.Port)
			require.True(t, retiring.Modified)
			require.Equal(t, test.deleteErr == nil, retiring.Deleted)
			require.Equal(t, survivorBefore, *surviving)
			require.Same(t, surviving, backend.HAProxySrvs[surviving.Name])
			require.False(t, backend.DynUpdateFailed)

			rt.calls = nil
			backend.Endpoints = store.RuntimeEndpoints{a: {}, b: {}}
			require.NoError(t, client.SyncBackendSrvs(backend))
			var wantCalls []string
			if test.deleteErr == nil {
				wantCalls = append(wantCalls,
					"add server "+target+" 192.0.2.1:8080",
					"enable server "+target,
				)
				require.NotSame(t, retiring, backend.HAProxySrvs[retiring.Name])
			} else {
				require.Same(t, retiring, backend.HAProxySrvs[retiring.Name])
			}
			wantCalls = append(wantCalls, "set server "+target+" addr 192.0.2.1 port 8080;set server "+target+" state ready;")
			require.Equal(t, wantCalls, rt.calls)
			require.Equal(t, &store.HAProxySrv{
				Name: retiring.Name, Address: a.Address, Port: a.Port, Modified: true,
			}, backend.HAProxySrvs[retiring.Name])
			require.Len(t, backend.HAProxySrvs, 2)
			require.Equal(t, survivorBefore, *surviving)
			require.Same(t, surviving, backend.HAProxySrvs[surviving.Name])
			require.False(t, backend.DynUpdateFailed)
		})
	}
}

type retirementClient struct {
	clientnative.HAProxyClient
	rt runtime.Runtime
}

func (c retirementClient) Runtime() (runtime.Runtime, error) {
	return c.rt, nil
}

func (c retirementClient) Configuration() (configuration.Configuration, error) {
	return retirementConfiguration{}, nil
}

type retirementConfiguration struct {
	configuration.Configuration
}

func (retirementConfiguration) GetDefaultsSection(string, string) (int64, *models.Defaults, error) {
	return 0, nil, nil
}

// Record the native API boundary, not HAProxy's routing or connection behavior.
type retirementRuntime struct {
	runtime.Runtime
	calls     []string
	deleteErr error
}

func (r *retirementRuntime) ExecuteRaw(command string) (string, error) {
	r.calls = append(r.calls, command)
	return "", nil
}

func (r *retirementRuntime) DeleteServer(backend, server string) error {
	r.calls = append(r.calls, "del server "+backend+"/"+server)
	return r.deleteErr
}

func (r *retirementRuntime) AddServer(backend, server, args string) error {
	r.calls = append(r.calls, fmt.Sprintf("add server %s/%s %s", backend, server, args))
	return nil
}

func (r *retirementRuntime) EnableServer(backend, server string) error {
	r.calls = append(r.calls, "enable server "+backend+"/"+server)
	return nil
}
