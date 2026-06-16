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
	"os"
	"testing"

	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// TestGlobalCfgConvergesWithoutLogConfig reproduces the periodic-reload bug: a fresh
// deployment with no log configuration reloaded on every sync because the injected
// default log target kept the global diff non-empty forever.
func TestGlobalCfgConvergesWithoutLogConfig(t *testing.T) {
	c := buildGlobalTestController(t)

	// First sync legitimately reloads (sets defaults + default log target).
	require.NoError(t, c.haproxy.APIStartTransaction())
	c.globalCfg()
	require.NoError(t, c.haproxy.APICommitTransaction())
	c.haproxy.APIDisposeTransaction()
	instance.Reset()

	// Steady-state sync: nothing changed, so no reload must be required.
	require.NoError(t, c.haproxy.APIStartTransaction())
	c.globalCfg()
	require.NoError(t, c.haproxy.APICommitTransaction())
	c.haproxy.APIDisposeTransaction()

	require.False(t, instance.NeedReload(),
		"steady-state globalCfg must not request a reload when nothing changed")
}

func buildGlobalTestController(t *testing.T) *HAProxyController {
	t.Helper()
	tempDir := t.TempDir()

	var osArgs utils.OSArgs
	os.Args = []string{os.Args[0], "-e", "-t", "--config-dir=" + tempDir}
	parser := flags.NewParser(&osArgs, flags.IgnoreUnknown)
	_, err := parser.Parse()
	require.NoError(t, err)

	cfg, err := os.ReadFile("../../fs/usr/local/etc/haproxy/haproxy.cfg")
	require.NoError(t, err)

	haproxyEnv := env.Env{
		CfgDir: tempDir,
		Proxies: env.Proxies{
			FrontHTTP:  "http",
			FrontHTTPS: "https",
			FrontSSL:   "ssl",
			BackSSL:    "ssl-backend",
		},
	}

	h, err := haproxy.New(osArgs, haproxyEnv, cfg, nil, nil, rules.New())
	require.NoError(t, err)

	return &HAProxyController{
		osArgs:       osArgs,
		haproxy:      h,
		podNamespace: os.Getenv("POD_NAMESPACE"),
		store:        store.NewK8sStore(osArgs),
		annotations:  annotations.New(),
	}
}
