package api

import (
	"errors"
	"strings"
	"testing"

	clientnative "github.com/haproxytech/client-native/v6"
	"github.com/haproxytech/client-native/v6/configuration"
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/client-native/v6/runtime"
	rutracker "github.com/haproxytech/kubernetes-ingress/pkg/runtime-update-tracker"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/stretchr/testify/require"
)

func TestBackendActivationPhases(t *testing.T) {
	for _, test := range []struct {
		name     string
		failCall int
		reply    string
	}{
		{"return before retirement", 0, ""},
		{"activation transport error", 1, ""},
		{"activation native error", 1, "[2]: activation failed"},
		{"retirement transport error", 2, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			tracker := rutracker.GetRuntimeUpdateTracker()
			tracker.Reset()
			t.Cleanup(tracker.Reset)
			a := store.RuntimeEndpoint{Address: "192.0.2.1", Port: 8080}
			b := store.RuntimeEndpoint{Address: "192.0.2.2", Port: 8080}
			backend := &store.RuntimeBackend{
				Name: "be", Endpoints: store.RuntimeEndpoints{a: {}},
				HAProxySrvs: map[string]*store.HAProxySrv{
					a.ComputeServerName(): {Name: a.ComputeServerName(), Address: "", Port: 1},
					b.ComputeServerName(): {Name: b.ComputeServerName(), Address: b.Address, Port: b.Port},
				},
			}
			rt := &activationCommands{failCall: test.failCall, reply: test.reply}
			client := &clientNative{
				nativeAPI: backendActivationClient{rt: rt},
				backends:  map[string]Backend{"be": {Backend: models.Backend{BackendBase: models.BackendBase{Name: "be"}}}},
			}
			err := client.SyncBackendSrvs(backend)
			if test.failCall == 0 {
				require.NoError(t, err)
				require.False(t, backend.DynUpdateFailed)
			} else {
				require.Error(t, err)
				require.True(t, backend.DynUpdateFailed)
			}
			want := []string{"set server be/" + a.ComputeServerName() + " addr 192.0.2.1 port 8080;set server be/" + a.ComputeServerName() + " state ready;"}
			if test.failCall != 1 {
				want = append(want, "set server be/"+b.ComputeServerName()+" addr 127.0.0.1 port 1;set server be/"+b.ComputeServerName()+" state maint;")
			}
			require.Equal(t, want, rt.commands)
			if test.failCall == 0 {
				require.Equal(t, []string{b.ComputeServerName()}, rt.deleted)
			} else {
				require.Empty(t, rt.deleted)
			}
		})
	}
}

func TestBackendActivationLaterChunkFailure(t *testing.T) {
	for _, reply := range []string{"", "[2]: activation failed"} {
		t.Run(reply, func(t *testing.T) {
			tracker := rutracker.GetRuntimeUpdateTracker()
			tracker.Reset()
			t.Cleanup(tracker.Reset)
			backend := &store.RuntimeBackend{Name: "be", Endpoints: store.RuntimeEndpoints{}, HAProxySrvs: map[string]*store.HAProxySrv{}}
			for i := range 400 {
				endpoint := store.RuntimeEndpoint{Address: "192.0.2.1", Port: int64(8080 + i)}
				backend.Endpoints[endpoint] = struct{}{}
				backend.HAProxySrvs[endpoint.ComputeServerName()] = &store.HAProxySrv{Name: endpoint.ComputeServerName(), Port: 1}
			}
			backend.HAProxySrvs["retiring"] = &store.HAProxySrv{Name: "retiring", Address: "192.0.2.2", Port: 8080}
			rt := &activationCommands{failCall: 2, reply: reply}
			client := &clientNative{
				nativeAPI: backendActivationClient{rt: rt},
				backends:  map[string]Backend{"be": {Backend: models.Backend{BackendBase: models.BackendBase{Name: "be"}}}},
			}
			require.Error(t, client.SyncBackendSrvs(backend))
			require.True(t, backend.DynUpdateFailed)
			require.Len(t, rt.commands, 2)
			for _, command := range rt.commands {
				require.True(t, len(command) < BufferSize)
				require.Contains(t, command, " state ready;")
				require.NotContains(t, command, " state maint;")
			}
			require.Greater(t, len(strings.Join(rt.commands, "")), BufferSize)
			require.Empty(t, rt.deleted)
		})
	}
}

func TestBackendActivationEmptyEndpoints(t *testing.T) {
	tracker := rutracker.GetRuntimeUpdateTracker()
	tracker.Reset()
	t.Cleanup(tracker.Reset)
	backend := &store.RuntimeBackend{Name: "be", Endpoints: store.RuntimeEndpoints{}, HAProxySrvs: map[string]*store.HAProxySrv{
		"retiring": {Name: "retiring", Address: "192.0.2.1", Port: 8080},
	}}
	rt := &activationCommands{}
	client := &clientNative{
		nativeAPI: backendActivationClient{rt: rt},
		backends:  map[string]Backend{"be": {Backend: models.Backend{BackendBase: models.BackendBase{Name: "be"}}}},
	}
	require.NoError(t, client.SyncBackendSrvs(backend))
	require.Equal(t, []string{"set server be/retiring addr 127.0.0.1 port 1;set server be/retiring state maint;"}, rt.commands)
	require.Equal(t, []string{"retiring"}, rt.deleted)
	require.False(t, backend.DynUpdateFailed)
}

type backendActivationClient struct {
	clientnative.HAProxyClient
	rt runtime.Runtime
}

func (c backendActivationClient) Runtime() (runtime.Runtime, error) { return c.rt, nil }
func (backendActivationClient) Configuration() (configuration.Configuration, error) {
	return backendActivationConfiguration{}, nil
}

type backendActivationConfiguration struct{ configuration.Configuration }

func (backendActivationConfiguration) GetDefaultsSection(string, string) (int64, *models.Defaults, error) {
	return 0, nil, nil
}

type activationCommands struct {
	runtime.Runtime
	commands []string
	deleted  []string
	failCall int
	reply    string
}

func (r *activationCommands) ExecuteRaw(command string) (string, error) {
	r.commands = append(r.commands, command)
	if len(r.commands) == r.failCall {
		if r.reply != "" {
			return r.reply, nil
		}
		return "", errors.New("injected runtime transport failure")
	}
	return "", nil
}

func (r *activationCommands) DeleteServer(backend, server string) error {
	r.deleted = append(r.deleted, server)
	return nil
}
