package store

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var backendRef = CRRef{Kind: CRKindBackend, Namespace: "ns", Name: "be"}

func backendCR(generation int64) *v3.Backend {
	return &v3.Backend{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "be", Generation: generation}}
}

func TestCRStartsEnabled(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "be", backendCR(1))

	require.False(t, k.CRDisabled(backendRef))
}

func TestDisableCRDisablesTheStoredGeneration(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "be", backendCR(1))

	generation, ok := k.DisableCR(backendRef)

	require.True(t, ok)
	require.EqualValues(t, 1, generation)
	require.True(t, k.CRDisabled(backendRef))
}

func TestDisableCRIgnoresUnknownResources(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})

	_, ok := k.DisableCR(CRRef{Kind: CRKindBackend, Namespace: "ns", Name: "missing"})

	require.False(t, ok)
}

func TestCRNewGenerationIsRetried(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "be", backendCR(1))
	k.DisableCR(backendRef)

	k.EventBackendCR("ns", "be", backendCR(2))

	require.False(t, k.CRDisabled(backendRef))
}

func TestCRSameGenerationStaysDisabled(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "be", backendCR(1))
	k.DisableCR(backendRef)

	// Informer resyncs replay the object unchanged.
	k.EventBackendCR("ns", "be", backendCR(1))

	require.True(t, k.CRDisabled(backendRef))
}

func TestCRDeletionForgetsTheRejection(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "be", backendCR(1))
	k.DisableCR(backendRef)

	k.EventBackendCR("ns", "be", nil)
	// A recreated resource restarts at generation 1.
	k.EventBackendCR("ns", "be", backendCR(1))

	require.False(t, k.CRDisabled(backendRef))
}

func TestCRKindsAreTrackedApart(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "shared", &v3.Backend{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "shared", Generation: 1}})
	k.EventFrontendCR("ns", "shared", &v3.Frontend{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "shared", Generation: 1}})

	k.DisableCR(CRRef{Kind: CRKindFrontend, Namespace: "ns", Name: "shared"})

	require.False(t, k.CRDisabled(CRRef{Kind: CRKindBackend, Namespace: "ns", Name: "shared"}))
	require.True(t, k.CRDisabled(CRRef{Kind: CRKindFrontend, Namespace: "ns", Name: "shared"}))
}

func TestEveryCRKindTracksItsGeneration(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	meta := metav1.ObjectMeta{Namespace: "ns", Name: "cr", Generation: 4}
	k.EventGlobalCR("ns", "cr", &v3.Global{ObjectMeta: meta})
	k.EventDefaultsCR("ns", "cr", &v3.Defaults{ObjectMeta: meta})
	k.EventFrontendCR("ns", "cr", &v3.Frontend{ObjectMeta: meta})
	k.EventTCPCR("ns", "cr", &TCPs{Status: ADDED, Namespace: "ns", Name: "cr", Generation: 4})

	for _, kind := range []CRKind{CRKindGlobal, CRKindDefaults, CRKindFrontend, CRKindTCP} {
		generation, ok := k.DisableCR(CRRef{Kind: kind, Namespace: "ns", Name: "cr"})
		require.True(t, ok, kind)
		require.EqualValues(t, 4, generation, kind)
	}
}

func TestTCPCRDeletionForgetsTheRejection(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	ref := CRRef{Kind: CRKindTCP, Namespace: "ns", Name: "cr"}
	k.EventTCPCR("ns", "cr", &TCPs{Status: ADDED, Namespace: "ns", Name: "cr", Generation: 1})
	k.DisableCR(ref)

	k.EventTCPCR("ns", "cr", &TCPs{Status: DELETED, Namespace: "ns", Name: "cr"})

	require.False(t, k.CRDisabled(ref))
}

func TestCRRefReadsAsKindAndPath(t *testing.T) {
	require.Equal(t, "Backend 'ns/be'", backendRef.String())
}

func TestTCPCREditOfADisabledResourceRequiresASync(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	ref := CRRef{Kind: CRKindTCP, Namespace: "ns", Name: "cr"}
	k.EventTCPCR("ns", "cr", &TCPs{Status: ADDED, Namespace: "ns", Name: "cr", Generation: 1})
	k.DisableCR(ref)

	// Same content, new generation: the resource must be rendered again.
	require.True(t, k.EventTCPCR("ns", "cr", &TCPs{Status: MODIFIED, Namespace: "ns", Name: "cr", Generation: 2}))
	require.False(t, k.CRDisabled(ref))
}

func TestTCPCRReplayedAsAddedWithANewGenerationRequiresASync(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	ref := CRRef{Kind: CRKindTCP, Namespace: "ns", Name: "cr"}
	k.EventTCPCR("ns", "cr", &TCPs{Status: ADDED, Namespace: "ns", Name: "cr", Generation: 1})
	k.DisableCR(ref)

	require.True(t, k.EventTCPCR("ns", "cr", &TCPs{Status: ADDED, Namespace: "ns", Name: "cr", Generation: 2}))
	require.False(t, k.CRDisabled(ref))
}

func TestRejectedCRsListsOnlyDisabledResources(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "bad", backendCR(2))
	k.EventBackendCR("ns", "good", &v3.Backend{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "good", Generation: 5}})
	k.DisableCR(CRRef{Kind: CRKindBackend, Namespace: "ns", Name: "bad"})

	require.Equal(t, map[CRRef]int64{{Kind: CRKindBackend, Namespace: "ns", Name: "bad"}: 2}, k.RejectedCRs())
}
