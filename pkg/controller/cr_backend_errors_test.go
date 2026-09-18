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
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

const rejectedCfg = `global
  daemon

backend ns_svc_app_http from haproxytech
  mode http
  errorfile 503 /nonexistent
`

func rejectedConfigError(t *testing.T) (dir string, configErr error) {
	dir = t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "haproxy.cfg"), []byte(rejectedCfg), 0o600))
	configErr = errors.New("config : parsing [/etc/haproxy/haproxy.cfg:6] : errorfile : error opening file '/nonexistent'")
	return dir, configErr
}

func controllerWithBackendCR(t *testing.T, fromCR bool) *HAProxyController {
	k := store.NewK8sStore(utils.OSArgs{})
	k.EventBackendCR("ns", "be", &v3.Backend{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "be", Generation: 3}})
	if fromCR {
		k.BackendsFromCR["ns_svc_app_http"] = "ns/be"
	}
	return &HAProxyController{store: k}
}

func TestRejectedBackendDisablesItsCustomResource(t *testing.T) {
	c := controllerWithBackendCR(t, true)
	dir, configErr := rejectedConfigError(t)

	require.True(t, c.disableRejectedBackendCRs(configErr, dir))
	require.True(t, c.store.BackendCRDisabled("ns", "be"))
}

func TestRejectedBackendWithoutCustomResourceIsLeftAlone(t *testing.T) {
	c := controllerWithBackendCR(t, false)
	dir, configErr := rejectedConfigError(t)

	require.False(t, c.disableRejectedBackendCRs(configErr, dir))
	require.False(t, c.store.BackendCRDisabled("ns", "be"))
}

func TestCleanForgetsWhichBackendsCameFromCustomResources(t *testing.T) {
	c := buildGlobalTestController(t)
	c.store.BackendsFromCR["ns_svc_app_http"] = "ns/be"

	c.clean(true)

	require.Empty(t, c.store.BackendsFromCR, "a failed sync must not leave attributions for the next one")
}
