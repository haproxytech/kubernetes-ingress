package k8stransform

import (
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/stretchr/testify/require"
)

// Server names are now hashes; a use-server rule can't target them, so the field is dropped at ingestion.
func TestTransformBackendDropsServerSwitchingRules(t *testing.T) {
	in := &v3.Backend{}
	in.Name = "be"
	in.Spec.ServerSwitchingRuleList = models.ServerSwitchingRules{
		{TargetServer: "SRV_1", Cond: "if", CondTest: "is_admin"},
	}

	out, err := TransformBackend(in)

	require.NoError(t, err)
	require.Empty(t, out.(*v3.Backend).Spec.ServerSwitchingRuleList)
}
