package handler

import (
	"errors"
	"fmt"
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// Only frontend operations are implemented: parse failures must not build backends.
type tcpSelectorClient struct {
	api.HAProxyClient
	frontends models.Frontends
	deleteErr error
	bindErr   error
	getErr    error
	deletes   []string
}

func (c *tcpSelectorClient) FrontendsGet() (models.Frontends, error) {
	return c.frontends, nil
}

func (c *tcpSelectorClient) FrontendGet(name string) (models.Frontend, error) {
	if c.getErr != nil {
		return models.Frontend{}, c.getErr
	}
	for _, frontend := range c.frontends {
		if frontend.Name == name {
			return *frontend, nil
		}
	}
	return models.Frontend{}, fmt.Errorf("frontend %s not found", name)
}

func (c *tcpSelectorClient) FrontendDelete(name string) error {
	c.deletes = append(c.deletes, name)
	if c.deleteErr != nil {
		return c.deleteErr
	}
	for i, frontend := range c.frontends {
		if frontend.Name == name {
			c.frontends = append(c.frontends[:i], c.frontends[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("frontend %s not found", name)
}

func (c *tcpSelectorClient) FrontendBindsGet(string) (models.Binds, error) {
	return models.Binds{{Name: "v4", Address: "0.0.0.0:9000"}}, c.bindErr
}

func TestTCPServicesSelectorEligibility(t *testing.T) {
	for _, test := range []struct {
		name      string
		input     string
		selector  bool
		relevant  bool
		missingNS bool
		deleteErr error
		bindErr   error
		getErr    error
		withdraw  bool
	}{
		{name: "dormant", input: "app/echo:80", selector: true, withdraw: true},
		{name: "retired", input: "app/echo:80", selector: true, missingNS: true, withdraw: true},
		{name: "delete failure", input: "app/echo:80", selector: true, deleteErr: errors.New("delete failed")},
		{name: "frontend lookup failure", input: "app/echo:80", selector: true, getErr: errors.New("frontend lookup failed")},
		{name: "dormant incorrect format", input: "app/echo", selector: true},
		{name: "incorrect format", input: "app/echo", selector: true, relevant: true},
		{name: "incorrect service name", input: "echo:80", selector: true, relevant: true},
		{name: "missing service", input: "app/missing:80", selector: true, relevant: true},
		{name: "invalid port", input: "app/echo:invalid", selector: true, relevant: true},
		{name: "legacy missing namespace", input: "app/echo:80", missingNS: true},
		{name: "legacy malformed", input: "app/echo"},
		{name: "bind API failure", input: "app/echo:80", selector: true, relevant: true, bindErr: errors.New("bind lookup failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			previous := instance.DefaultConfigurationManager
			instance.DefaultConfigurationManager = instance.NewConfigurationManager()
			t.Cleanup(func() { instance.DefaultConfigurationManager = previous })

			k := store.NewK8sStore(utils.OSArgs{})
			ns := k.GetNamespace("app")
			ns.Services["echo"] = &store.Service{Namespace: "app", Name: "echo"}
			k.NamespacesAccess.LabelSelectorActive = test.selector
			ns.Relevant = test.relevant
			if test.missingNS {
				delete(k.Namespaces, "app")
			}
			k.ConfigMaps.TCPServices.Annotations = map[string]string{"9000": test.input}
			original := models.Frontend{FrontendBase: models.FrontendBase{Name: "tcp-9000", DefaultBackend: "app_svc_echo_http"}}
			unrelated := models.Frontend{FrontendBase: models.FrontendBase{Name: "http", DefaultBackend: "other_backend"}}
			client := &tcpSelectorClient{frontends: models.Frontends{&original, &unrelated}, deleteErr: test.deleteErr, bindErr: test.bindErr, getErr: test.getErr}

			require.NoError(t, (TCPServices{}).Update(k, haproxy.HAProxy{HAProxyClient: client}, nil))
			client.getErr = nil
			frontend, err := client.FrontendGet("tcp-9000")
			if test.withdraw {
				require.Error(t, err, "selector-ineligible namespace must not retain a reachable frontend")
			} else {
				require.NoError(t, err)
				require.Equal(t, original, frontend)
			}
			require.Equal(t, test.withdraw, instance.NeedReload())
			if test.withdraw || test.deleteErr != nil {
				require.Equal(t, []string{"tcp-9000"}, client.deletes)
			} else {
				require.Empty(t, client.deletes)
			}
			other, err := client.FrontendGet("http")
			require.NoError(t, err)
			require.Equal(t, unrelated, other)
			require.Equal(t, map[string]string{"9000": test.input}, k.ConfigMaps.TCPServices.Annotations)
		})
	}
}
