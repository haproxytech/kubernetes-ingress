// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ingress

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// passthroughStore returns a store holding one service carrying svcAnn, a configmap
// carrying cfgAnn, and a path pointing at that service.
func passthroughStore(svcAnn, cfgAnn map[string]string) (store.K8s, *store.IngressPath) {
	k := store.NewK8sStore(utils.OSArgs{})
	ns := k.GetNamespace("ns")
	ns.Services["svc"] = &store.Service{
		Namespace:   "ns",
		Name:        "svc",
		Status:      store.ADDED,
		Annotations: svcAnn,
	}
	k.ConfigMaps.Main.Annotations = cfgAnn
	return k, &store.IngressPath{SvcNamespace: "ns", SvcName: "svc", SvcPortString: "http"}
}

// TestServiceScopeWinsOverIngressScope is the point of the whole change, and the
// direction matters: the service must be able to say "my pod terminates TLS itself" and
// have it hold for every ingress routing to it, including one which says otherwise. The
// reverse case is the one that used to be impossible to express at all - the service
// could not turn passthrough off for an ingress asking for it.
func TestServiceScopeWinsOverIngressScope(t *testing.T) {
	k, path := passthroughStore(map[string]string{"ssl-passthrough": "true"}, nil)
	enabled, err := SSLPassthroughEnabled(k, path, map[string]string{"ssl-passthrough": "false"})
	require.NoError(t, err)
	require.True(t, enabled, "the service must be able to turn passthrough on for an ingress which did not ask for it")

	k, path = passthroughStore(map[string]string{"ssl-passthrough": "false"}, nil)
	enabled, err = SSLPassthroughEnabled(k, path, map[string]string{"ssl-passthrough": "true"})
	require.NoError(t, err)
	require.False(t, enabled, "the service must be able to turn passthrough off for an ingress which asked for it")
}

// TestIngressScopeStillApplies pins backwards compatibility: the annotation on the
// ingress keeps working when the service says nothing, which is how every existing
// deployment uses it.
func TestIngressScopeStillApplies(t *testing.T) {
	k, path := passthroughStore(nil, nil)
	enabled, err := SSLPassthroughEnabled(k, path, map[string]string{"ssl-passthrough": "true"})
	require.NoError(t, err)
	require.True(t, enabled)
}

// TestConfigMapScopeIsTheLastResort covers the third scope, and the default: with nobody
// declaring anything the answer is false, which is what keeps the layer 4 topology off.
func TestConfigMapScopeIsTheLastResort(t *testing.T) {
	k, path := passthroughStore(nil, map[string]string{"ssl-passthrough": "true"})
	enabled, err := SSLPassthroughEnabled(k, path, nil)
	require.NoError(t, err)
	require.True(t, enabled, "the configmap value must apply when neither the service nor the ingress declares one")

	k, path = passthroughStore(nil, nil)
	enabled, err = SSLPassthroughEnabled(k, path, nil)
	require.NoError(t, err)
	require.False(t, enabled, "no scope declaring it must leave passthrough off")
}

// TestMissingServiceFallsBackToTheOtherScopes matters because this resolution now runs
// for every path, including those of an ingress pointing at a service which does not
// exist - a common transient state, and one the resolution must not turn into a panic or
// into a silent loss of the ingress value. Reporting the missing service is service.New's
// job, not this one's.
func TestMissingServiceFallsBackToTheOtherScopes(t *testing.T) {
	k, _ := passthroughStore(nil, nil)
	path := &store.IngressPath{SvcNamespace: "ns", SvcName: "absent", SvcPortString: "http"}

	enabled, err := SSLPassthroughEnabled(k, path, map[string]string{"ssl-passthrough": "true"})
	require.NoError(t, err)
	require.True(t, enabled)
}

// TestUnparsableValueIsReportedAndDefaultsToFalse pins the contract the callers rely on:
// they log the error and carry on with the returned bool, so it has to be the safe
// default rather than an undefined value.
func TestUnparsableValueIsReportedAndDefaultsToFalse(t *testing.T) {
	k, path := passthroughStore(map[string]string{"ssl-passthrough": "yes please"}, nil)
	enabled, err := SSLPassthroughEnabled(k, path, nil)
	require.Error(t, err)
	require.False(t, enabled)
}
