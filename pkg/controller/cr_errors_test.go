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

package controller

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

const rejectedCfg = `global
  daemon
  tune.bufsize 0

backend ns_svc_app_http from haproxytech
  mode http
  errorfile 503 /nonexistent
`

var (
	backendRef = store.CRRef{Kind: store.CRKindBackend, Namespace: "ns", Name: "be"}
	globalRef  = store.CRRef{Kind: store.CRKindGlobal, Namespace: "ns", Name: "gl"}
)

func rejectedConfig(t *testing.T, line int) (dir string, configErr error) {
	dir = t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "haproxy.cfg"), []byte(rejectedCfg), 0o600))
	configErr = errors.New("config : parsing [/etc/haproxy/haproxy.cfg:" + string(rune('0'+line)) + "] : rejected")
	return dir, configErr
}

func controllerWithCRs() *HAProxyController {
	k := store.NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "be", &v3.Backend{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "be", Generation: 3}})
	k.EventGlobalCR("ns", "gl", &v3.Global{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "gl", Generation: 2}})
	return &HAProxyController{store: k}
}

func TestRejectedBackendDisablesItsCustomResource(t *testing.T) {
	c := controllerWithCRs()
	c.store.SectionsFromCR["backend/ns_svc_app_http"] = backendRef
	dir, configErr := rejectedConfig(t, 7)

	require.True(t, c.disableRejectedCRs(configErr, dir))
	require.True(t, c.store.CRDisabled(backendRef))
}

func TestRejectedSectionWithoutCustomResourceIsLeftAlone(t *testing.T) {
	c := controllerWithCRs()
	dir, configErr := rejectedConfig(t, 7)

	require.False(t, c.disableRejectedCRs(configErr, dir))
	require.False(t, c.store.CRDisabled(backendRef))
}

func TestRejectedGlobalFallsBackToAnnotations(t *testing.T) {
	c := controllerWithCRs()
	c.store.SectionsFromCR[annotations.Section{Kind: annotations.SectionGlobal}.Key()] = globalRef
	dir, configErr := rejectedConfig(t, 3)

	require.True(t, c.disableRejectedCRs(configErr, dir))
	require.True(t, c.store.CRDisabled(globalRef))
}

func TestCleanForgetsWhichSectionsCameFromCustomResources(t *testing.T) {
	c := buildGlobalTestController(t)
	c.store.SectionsFromCR["backend/ns_svc_app_http"] = backendRef

	c.clean(true)

	require.Empty(t, c.store.SectionsFromCR, "a failed sync must not leave attributions for the next one")
}
