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

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/handler"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
)

// newSSLPassthroughHandler builds the HTTPS handler with the minimal binding
// configuration the ssl-passthrough code path needs.
func newSSLPassthroughHandler(t *testing.T) *handler.HTTPS {
	t.Helper()
	return &handler.HTTPS{
		Enabled:  true,
		IPv4:     true,
		AddrIPv4: "0.0.0.0",
		Port:     8443,
	}
}

// runSSLPassthroughSync mimics one reconciliation of updateHAProxy() restricted to
// the HTTPS handler: open a transaction, run the handler, then final-commit, which
// is where BackendDeleteAllUnnecessary garbage-collects unused backends.
func runSSLPassthroughSync(t *testing.T, c *HAProxyController, h *handler.HTTPS) {
	t.Helper()
	require.NoError(t, c.haproxy.APIStartTransaction())
	require.NoError(t, h.Update(c.store, c.haproxy, c.annotations))
	require.NoError(t, c.haproxy.APIFinalCommitTransaction())
	c.haproxy.APIDisposeTransaction()
}

// runFailingSSLPassthroughSync is runSSLPassthroughSync for a sync the handler is
// expected to report an error for. The final commit still runs, because that is what
// the controller does: a handler error is logged, not fatal.
func runFailingSSLPassthroughSync(t *testing.T, c *HAProxyController, h *handler.HTTPS) {
	t.Helper()
	require.NoError(t, c.haproxy.APIStartTransaction())
	require.Error(t, h.Update(c.store, c.haproxy, c.annotations),
		"the sync was set up to fail: if it does not, the test no longer covers anything")
	require.NoError(t, c.haproxy.APIFinalCommitTransaction())
	c.haproxy.APIDisposeTransaction()
}

// requireSSLPassthroughChainIntact asserts the invariant HAProxy enforces when it
// parses its configuration: the backend a frontend defaults to must exist. When it
// does not, haproxy rejects the *whole* configuration with "unable to find required
// default_backend", the transaction is refused and everything it carried (routes,
// allow-list maps, annotations) is silently lost.
func requireSSLPassthroughChainIntact(t *testing.T, c *HAProxyController) {
	t.Helper()
	front, err := c.haproxy.FrontendGet(c.haproxy.FrontSSL)
	require.NoError(t, err, "the ssl-passthrough frontend must exist")
	require.Equal(t, c.haproxy.BackSSL, front.DefaultBackend)

	_, err = c.haproxy.BackendGet(front.DefaultBackend)
	require.NoError(t, err,
		"frontend %q defaults to backend %q, which must never be garbage-collected",
		c.haproxy.FrontSSL, front.DefaultBackend)

	cfg, err := os.ReadFile(c.haproxy.Env.MainCFGFile)
	require.NoError(t, err)
	require.Contains(t, string(cfg), "backend "+c.haproxy.BackSSL,
		"the committed configuration must still declare %q, otherwise haproxy refuses to start",
		c.haproxy.BackSSL)
}

// setupSSLPassthrough brings the controller to a steady state with ssl-passthrough
// enabled, and returns the handler to reuse for the following syncs.
func setupSSLPassthrough(t *testing.T, c *HAProxyController) *handler.HTTPS {
	t.Helper()
	setupFrontends(t, c)
	haproxy.SSLPassthrough = true
	t.Cleanup(func() { haproxy.SSLPassthrough = false })

	h := newSSLPassthroughHandler(t)
	runSSLPassthroughSync(t, c, h)
	requireSSLPassthroughChainIntact(t, c)
	instance.Reset()
	return h
}

// TestSSLPassthroughBackendSurvivesLostPermanence is the regression test for the
// controller getting permanently stuck once ssl-passthrough is enabled.
//
// The proxy-chaining backend is referenced only by the ssl frontend
// default_backend, never by an ingress path, so nothing marks it as used during a
// reconciliation: only its in-memory "permanent" flag keeps
// BackendDeleteAllUnnecessary from dropping it. That flag does not survive a
// controller restart nor a rollback after a failed transaction, and it used to be
// re-set only when the backend was first created. Every sync after the flag was
// lost produced a configuration whose ssl frontend pointed at a deleted backend,
// so haproxy rejected it and no ingress change could be applied any more.
func TestSSLPassthroughBackendSurvivesLostPermanence(t *testing.T) {
	c := buildGlobalTestController(t)
	h := setupSSLPassthrough(t, c)

	// Simulate the in-memory permanence being lost while the committed
	// configuration still declares the backend and the frontend still points at
	// it. BackendDelete leaves exactly that state: the entry is still known but
	// neither used nor permanent, so the next final commit would delete it.
	c.haproxy.BackendDelete(c.haproxy.BackSSL)

	runSSLPassthroughSync(t, c, h)
	requireSSLPassthroughChainIntact(t, c)
}

// TestSSLPassthroughBackendSurvivesFailedTransaction covers the same invariant
// through the path the controller actually takes when a sync fails: the backends
// state is snapshotted on success and restored on failure.
func TestSSLPassthroughBackendSurvivesFailedTransaction(t *testing.T) {
	c := buildGlobalTestController(t)
	h := setupSSLPassthrough(t, c)

	require.NoError(t, c.haproxy.PushPreviousBackends())
	require.NoError(t, c.haproxy.PopPreviousBackends())

	runSSLPassthroughSync(t, c, h)
	requireSSLPassthroughChainIntact(t, c)
}

// TestSSLPassthroughSteadyStateDoesNotReload guards the fix against reload churn:
// re-asserting the backend on every sync must be a no-op when nothing changed.
func TestSSLPassthroughSteadyStateDoesNotReload(t *testing.T) {
	c := buildGlobalTestController(t)
	h := setupSSLPassthrough(t, c)

	runSSLPassthroughSync(t, c, h)

	require.False(t, instance.NeedReload(),
		"steady-state ssl-passthrough sync must not request a reload")
}

// TestSSLPassthroughBackendTrackedAfterMemoryWipe covers the other way the
// controller loses the proxy-chaining backend: PopPreviousBackends() with no saved
// state wipes the backends map but leaves the frontends map untouched. The next
// sync therefore re-enters enableSSLPassthrough() on a frontend whose binds are
// already there, and aborting on "bind already exists" used to skip the backend
// creation, leaving the configuration declaring a backend the controller no longer
// tracks: its server list stops being reconciled, silently.
func TestSSLPassthroughBackendTrackedAfterMemoryWipe(t *testing.T) {
	c := buildGlobalTestController(t)
	h := setupSSLPassthrough(t, c)

	// No state was saved yet, so this takes the clear(c.backends) branch.
	require.NoError(t, c.haproxy.PopPreviousBackends())
	require.False(t, c.haproxy.BackendExists(c.haproxy.BackSSL),
		"the wipe must have dropped the backend from the in-memory state")

	runSSLPassthroughSync(t, c, h)

	require.True(t, c.haproxy.BackendExists(c.haproxy.BackSSL),
		"the sync must take the backend back under management")
	requireSSLPassthroughChainIntact(t, c)

	backend, err := c.haproxy.BackendGet(c.haproxy.BackSSL)
	require.NoError(t, err)
	require.Contains(t, backend.Servers, c.haproxy.FrontHTTPS,
		"the proxy-chaining server must be managed again too")
}

// TestSSLPassthroughBackendSurvivesUnrelatedHandlerError checks that the invariant is
// held even when the HTTPS handler gives up early on something unrelated.
//
// The handler does four independent jobs in one Update(): the certificate signer, the
// ssl-offload, the client TLS authentication and the ssl-passthrough. A malformed
// generate-certificates-signer annotation in the ConfigMap resolves to an "invalid
// format" error, which is not a not-found, so the handler returns before ever reaching
// its ssl-passthrough section. A single wrong character in an unrelated annotation
// would then stop the chaining backend from being re-asserted, and the final commit
// would delete it while the ssl frontend still declares it.
func TestSSLPassthroughBackendSurvivesUnrelatedHandlerError(t *testing.T) {
	c := buildGlobalTestController(t)
	h := setupSSLPassthrough(t, c)

	// Missing namespace: GetK8sPath() reports "invalid format", not a not-found.
	c.store.ConfigMaps.Main.Annotations = map[string]string{
		"generate-certificates-signer": "/some-secret",
	}
	c.haproxy.BackendDelete(c.haproxy.BackSSL)

	runFailingSSLPassthroughSync(t, c, h)
	requireSSLPassthroughChainIntact(t, c)
}
