package annotations

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Mirrors the layout client-native renders: section headers at column 0,
// directives indented, config snippets fenced by BEGIN/END comments.
const failedCfg = `global
  daemon

frontend http
  mode http
  http-request set-var(txn.base) base

backend appNs_svc_first_https from haproxytech
  mode http
  balance roundrobin
  errorfile 503 /nonexistent
  ###_config-snippet_### BEGIN
  ### service:appNs_svc_first_https/appNs/first ###
  http-send-name-header x-dst-server
  ###_config-snippet_### END
  server s1 127.0.0.1:80

backend appNs_svc_second_https from haproxytech
  mode http
  retries 7
`

func TestBackendsInErrorNamesTheBackendOwningTheLine(t *testing.T) {
	require.Equal(t, []string{"appNs_svc_first_https"}, backendsInError(failedCfg, []int{11}))
}

func TestBackendsInErrorSkipsLinesInsideConfigSnippets(t *testing.T) {
	require.Empty(t, backendsInError(failedCfg, []int{14}))
}

func TestBackendsInErrorSkipsLinesOutsideBackends(t *testing.T) {
	require.Empty(t, backendsInError(failedCfg, []int{2, 6}))
}

func TestBackendsInErrorReportsEachBackendOnce(t *testing.T) {
	require.Equal(t, []string{"appNs_svc_first_https", "appNs_svc_second_https"},
		backendsInError(failedCfg, []int{16, 11, 20}))
}

func TestBackendsInErrorIgnoresLinesBeyondTheFile(t *testing.T) {
	require.Empty(t, backendsInError(failedCfg, []int{0, 999}))
}

func TestBackendsInErrorReadsTheRejectedFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "haproxy.cfg"), []byte(failedCfg), 0o600))
	configErr := errors.New("config : parsing [/etc/haproxy/haproxy.cfg:11] : errorfile : error opening file '/nonexistent'.\n" +
		"config : Error(s) found in configuration file : /etc/haproxy/haproxy.cfg")

	backends, err := BackendsInError(configErr, dir)

	require.NoError(t, err)
	require.Equal(t, []string{"appNs_svc_first_https"}, backends)
}

func TestBackendsInErrorWithoutErrorReportsNothing(t *testing.T) {
	backends, err := BackendsInError(nil, t.TempDir())
	require.NoError(t, err)
	require.Empty(t, backends)
}
