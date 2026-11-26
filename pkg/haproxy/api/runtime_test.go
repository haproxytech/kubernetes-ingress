package api

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	clientnative "github.com/haproxytech/client-native/v6"
	"github.com/haproxytech/client-native/v6/runtime"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/stretchr/testify/require"
)

func TestSetServerAddrAndState(t *testing.T) {
	for _, test := range []struct {
		name    string
		server  RuntimeServerData
		command string
	}{
		{"retire placeholder", RuntimeServerData{"be", "s1", "127.0.0.1", "maint", 1}, "set server be/s1 state maint;"},
		{"retire real address", RuntimeServerData{"be", "s1", "192.0.2.1", "maint", 8080}, "set server be/s1 state maint;"},
		{"activate", RuntimeServerData{"be", "s1", "192.0.2.1", "ready", 8080}, "set server be/s1 addr 192.0.2.1 port 8080;set server be/s1 state ready;"},
		{"drain", RuntimeServerData{"be", "s1", "192.0.2.1", "drain", 8080}, "set server be/s1 addr 192.0.2.1 port 8080;set server be/s1 state drain;"},
		{"omit port", RuntimeServerData{"be", "s1", "2001:db8::1", "ready", 0}, "set server be/s1 addr 2001:db8::1;set server be/s1 state ready;"},
	} {
		t.Run(test.name, func(t *testing.T) {
			rt := &serverCommandRuntime{}
			client := &clientNative{nativeAPI: serverCommandClient{rt: rt}}
			require.NoError(t, client.SetServerAddrAndState([]RuntimeServerData{test.server}))
			require.Equal(t, []string{test.command}, rt.commands)
		})
	}
}

func TestSetServerAddrAndStateBatches(t *testing.T) {
	rt := &serverCommandRuntime{}
	client := &clientNative{nativeAPI: serverCommandClient{rt: rt}}
	require.NoError(t, client.SetServerAddrAndState(nil))
	require.Empty(t, rt.commands)

	var servers []RuntimeServerData
	var expected strings.Builder
	for i := 0; i < 600; i++ {
		name := fmt.Sprintf("s%d", i)
		if i%2 == 0 {
			servers = append(servers, RuntimeServerData{"be", name, "127.0.0.1", "maint", 1})
			fmt.Fprintf(&expected, "set server be/%s state maint;", name)
		} else {
			servers = append(servers, RuntimeServerData{"be", name, "192.0.2.1", "ready", 8080})
			fmt.Fprintf(&expected, "set server be/%s addr 192.0.2.1 port 8080;set server be/%s state ready;", name, name)
		}
	}
	require.NoError(t, client.SetServerAddrAndState(servers))
	require.Greater(t, len(rt.commands), 1)
	for _, command := range rt.commands {
		require.NotEmpty(t, command)
		require.Less(t, len(command), BufferSize)
	}
	want := strings.Split(expected.String(), ";")
	got := strings.Split(strings.Join(rt.commands, ""), ";")
	require.Len(t, got, len(want))
	for i, command := range want {
		require.Equal(t, command, got[i], "command %d", i)
	}
}

func TestSetServerAddrAndStateReturnsRuntimeError(t *testing.T) {
	failure := errors.New("runtime socket unavailable")
	rt := &serverCommandRuntime{err: failure}
	client := &clientNative{nativeAPI: serverCommandClient{rt: rt}}
	require.ErrorIs(t, client.SetServerAddrAndState([]RuntimeServerData{
		{BackendName: "be", ServerName: "s1", State: "maint"},
	}), failure)
	require.Len(t, rt.commands, 1)
}

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

type serverCommandClient struct {
	clientnative.HAProxyClient
	rt runtime.Runtime
}

func (c serverCommandClient) Runtime() (runtime.Runtime, error) {
	return c.rt, nil
}

type serverCommandRuntime struct {
	runtime.Runtime
	commands []string
	err      error
}

func (r *serverCommandRuntime) ExecuteRaw(command string) (string, error) {
	r.commands = append(r.commands, command)
	return "", r.err
}
