package store

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func backendCR(name string, generation int64) *v3.Backend {
	return &v3.Backend{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: name, Generation: generation}}
}

func TestBackendCRStartsEnabled(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "be", backendCR("be", 1))

	require.False(t, k.BackendCRDisabled("ns", "be"))
}

func TestDisableBackendCRDisablesTheStoredGeneration(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "be", backendCR("be", 1))

	generation, ok := k.DisableBackendCR("ns", "be")

	require.True(t, ok)
	require.EqualValues(t, 1, generation)
	require.True(t, k.BackendCRDisabled("ns", "be"))
}

func TestDisableBackendCRIgnoresUnknownResources(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})

	_, ok := k.DisableBackendCR("ns", "missing")

	require.False(t, ok)
	require.False(t, k.BackendCRDisabled("ns", "missing"))
}

func TestBackendCRNewGenerationIsRetried(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "be", backendCR("be", 1))
	k.DisableBackendCR("ns", "be")

	k.EventBackendCR("ns", "be", backendCR("be", 2))

	require.False(t, k.BackendCRDisabled("ns", "be"))
}

func TestBackendCRSameGenerationStaysDisabled(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "be", backendCR("be", 1))
	k.DisableBackendCR("ns", "be")

	// Informer resyncs replay the object unchanged.
	k.EventBackendCR("ns", "be", backendCR("be", 1))

	require.True(t, k.BackendCRDisabled("ns", "be"))
}

func TestBackendCRDeletionForgetsTheRejection(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "be", backendCR("be", 1))
	k.DisableBackendCR("ns", "be")

	k.EventBackendCR("ns", "be", nil)
	// A recreated resource restarts at generation 1.
	k.EventBackendCR("ns", "be", backendCR("be", 1))

	require.False(t, k.BackendCRDisabled("ns", "be"))
}
