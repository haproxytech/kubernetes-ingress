package controller

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/haproxytech/kubernetes-ingress/pkg/handler"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
)

func buildTCPSelectorController(t *testing.T) *HAProxyController {
	t.Helper()
	args := os.Args
	previous := instance.DefaultConfigurationManager
	instance.DefaultConfigurationManager = instance.NewConfigurationManager()
	t.Cleanup(func() {
		os.Args = args
		instance.DefaultConfigurationManager = previous
	})
	c := buildGlobalTestController(t)
	app := c.store.GetNamespace("app")
	other := c.store.GetNamespace("other")
	addBackendService(t, c.store, app, "echo")
	addBackendService(t, c.store, other, "echo")
	c.store.NamespacesAccess.LabelSelectorActive = true
	c.store.NamespacesAccess.Selector = labels.SelectorFromSet(labels.Set{"route": "enabled"})
	c.store.NamespacesAccess.Selected = map[string]struct{}{"app": {}, "other": {}}
	app.Labels = map[string]string{"route": "enabled"}
	other.Labels = map[string]string{"route": "enabled"}
	c.store.ConfigMaps.TCPServices.Annotations = map[string]string{
		"9000": "app/echo:80",
		"9001": "other/echo:80",
	}
	return c
}

func applyTCPSelectorPass(t *testing.T, c *HAProxyController) {
	t.Helper()
	instance.Reset()
	require.NoError(t, c.haproxy.APIStartTransaction())
	defer c.haproxy.APIDisposeTransaction()
	require.NoError(t, (handler.TCPServices{IPv4: true, AddrIPv4: "0.0.0.0"}).Update(c.store, c.haproxy, c.annotations))
	// Mirror the frontend publication phase: deletion is deferred until this pass,
	// then live frontends and backends are written to the actual configuration.
	require.NoError(t, c.haproxy.FrontendDeletePending())
	frontends, err := c.haproxy.FrontendsGet()
	require.NoError(t, err)
	for _, frontend := range frontends {
		if _, err := c.haproxy.FrontendGetStructured(frontend.Name); err != nil {
			require.NoError(t, c.haproxy.FrontendCreateStructured(frontend))
		} else {
			require.NoError(t, c.haproxy.FrontendEditStructured(frontend.Name, frontend))
		}
	}
	require.NoError(t, c.haproxy.APIFinalCommitTransaction())
}

func requireTCPSelectorRoute(t *testing.T, c *HAProxyController, frontendName, backendName, address string) {
	t.Helper()
	frontend, err := c.haproxy.FrontendGetStructured(frontendName)
	require.NoError(t, err)
	require.Equal(t, "tcp", frontend.Mode)
	require.Equal(t, backendName, frontend.DefaultBackend)
	require.Len(t, frontend.Binds, 1)
	bind := frontend.Binds["v4"]
	require.NotNil(t, bind.Port)
	require.Equal(t, address, fmt.Sprintf("%s:%d", bind.Address, *bind.Port))
	backend, err := c.haproxy.BackendGet(frontend.DefaultBackend)
	require.NoError(t, err)
	require.Equal(t, "tcp", backend.Mode)
	servers, err := c.haproxy.BackendServersGet(frontend.DefaultBackend)
	require.NoError(t, err)
	for _, server := range servers {
		if server.Address == "10.0.0.1" {
			require.NotNil(t, server.Port)
			require.EqualValues(t, 80, *server.Port)
			require.Equal(t, "disabled", server.Maintenance, "the endpoint must be usable, not a disabled placeholder")
			return
		}
	}
	t.Fatalf("backend %s has no usable service endpoint: %+v", backendName, servers)
}

func TestTCPServicesSelectorRestoration(t *testing.T) {
	c := buildTCPSelectorController(t)
	applyTCPSelectorPass(t, c)
	requireTCPSelectorRoute(t, c, "tcp-9000", "app_svc_echo_http", "0.0.0.0:9000")
	requireTCPSelectorRoute(t, c, "tcp-9001", "other_svc_echo_http", "0.0.0.0:9001")

	app := c.store.Namespaces["app"]
	app.Labels = map[string]string{}
	delete(c.store.NamespacesAccess.Selected, "app")
	require.True(t, c.store.SetNamespaceDormant("app"))
	applyTCPSelectorPass(t, c)
	_, err := c.haproxy.FrontendGetStructured("tcp-9000")
	require.Error(t, err, "Dormant namespace must no longer be reachable through its TCP frontend")
	require.True(t, instance.NeedReload(), "successful withdrawal must request a reload")
	requireTCPSelectorRoute(t, c, "tcp-9001", "other_svc_echo_http", "0.0.0.0:9001")
	require.Equal(t, "app/echo:80", c.store.ConfigMaps.TCPServices.Annotations["9000"])

	// Neither an unselected namespace nor a reselected but Starting session may restore traffic.
	applyTCPSelectorPass(t, c)
	_, err = c.haproxy.FrontendGetStructured("tcp-9000")
	require.Error(t, err)
	require.False(t, instance.NeedReload(), "an already withdrawn route must converge")
	app.Labels = map[string]string{"route": "enabled"}
	c.store.NamespacesAccess.Selected["app"] = struct{}{}
	applyTCPSelectorPass(t, c)
	_, err = c.haproxy.FrontendGetStructured("tcp-9000")
	require.Error(t, err, "Starting namespace must wait for READY")
	require.False(t, instance.NeedReload())

	require.True(t, c.store.MarkNamespaceReady("app"))
	applyTCPSelectorPass(t, c)
	require.True(t, instance.NeedReload())
	requireTCPSelectorRoute(t, c, "tcp-9000", "app_svc_echo_http", "0.0.0.0:9000")
	requireTCPSelectorRoute(t, c, "tcp-9001", "other_svc_echo_http", "0.0.0.0:9001")
	applyTCPSelectorPass(t, c)
	require.False(t, instance.NeedReload(), "restored route must converge")
}

func TestTCPServicesSelectorStartupIsolation(t *testing.T) {
	for _, state := range []string{"unselected", "starting", "always watched"} {
		t.Run(state, func(t *testing.T) {
			c := buildTCPSelectorController(t)
			c.store.Namespaces["app"].Relevant = false
			if state != "starting" {
				delete(c.store.NamespacesAccess.Selected, "app")
			}
			if state == "always watched" {
				c.store.NamespacesAccess.AlwaysSelected = map[string]struct{}{"app": {}}
			}
			applyTCPSelectorPass(t, c)
			_, err := c.haproxy.FrontendGetStructured("tcp-9000")
			require.Error(t, err)
			requireTCPSelectorRoute(t, c, "tcp-9001", "other_svc_echo_http", "0.0.0.0:9001")
			require.Equal(t, "app/echo:80", c.store.ConfigMaps.TCPServices.Annotations["9000"])
		})
	}
}

func TestTCPServicesSelectorLegacyEligibility(t *testing.T) {
	c := buildTCPSelectorController(t)
	c.store.NamespacesAccess.LabelSelectorActive = false
	c.store.Namespaces["app"].Relevant = false
	applyTCPSelectorPass(t, c)
	requireTCPSelectorRoute(t, c, "tcp-9000", "app_svc_echo_http", "0.0.0.0:9000")
}
