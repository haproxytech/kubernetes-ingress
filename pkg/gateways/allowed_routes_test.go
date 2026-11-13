package gateway

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// recordingStatus only records refusals; other calls are not expected.
type recordingStatus struct {
	StatusManager
	refused int
}

func (s *recordingStatus) SetRouteReasonNotAllowedByListeners(string, store.ParentRef) {
	s.refused++
}

func TestTCPRouteAllowedWithoutListenerRules(t *testing.T) {
	status := &recordingStatus{}
	gm := GatewayManagerImpl{statusManager: status}
	listener := store.Listener{Name: "tcp", GwNamespace: "gw-ns", GwName: "gw"}

	// A listener without rules only accepts routes from its own namespace.
	require.True(t, gm.isTCPRouteAllowedByListener(listener, "gw-ns", "gw-ns", store.ParentRef{}))
	require.Zero(t, status.refused)
	require.False(t, gm.isTCPRouteAllowedByListener(listener, "tenant-ns", "gw-ns", store.ParentRef{}))
	require.Equal(t, 1, status.refused)

	same := "Same"
	listener.AllowedRoutes = &store.AllowedRoutes{Namespaces: &store.RouteNamespaces{From: &same}}
	require.True(t, gm.isTCPRouteAllowedByListener(listener, "gw-ns", "gw-ns", store.ParentRef{}))
	require.False(t, gm.isTCPRouteAllowedByListener(listener, "tenant-ns", "gw-ns", store.ParentRef{}))
}
