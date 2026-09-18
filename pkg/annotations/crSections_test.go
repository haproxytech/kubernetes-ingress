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

defaults haproxytech
  retries 3

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

peers localinstance
  peer local 127.0.0.1:10000
`

func TestSectionsInErrorNamesTheBackendOwningTheLine(t *testing.T) {
	require.Equal(t, []Section{{Kind: SectionBackend, Name: "appNs_svc_first_https"}}, sectionsInError(failedCfg, []int{14}))
}

func TestSectionsInErrorNamesTheGlobalSection(t *testing.T) {
	require.Equal(t, []Section{{Kind: SectionGlobal}}, sectionsInError(failedCfg, []int{2}))
}

func TestSectionsInErrorNamesTheDefaultsSection(t *testing.T) {
	require.Equal(t, []Section{{Kind: SectionDefaults, Name: "haproxytech"}}, sectionsInError(failedCfg, []int{5}))
}

func TestSectionsInErrorNamesTheFrontendSection(t *testing.T) {
	require.Equal(t, []Section{{Kind: SectionFrontend, Name: "http"}}, sectionsInError(failedCfg, []int{9}))
}

func TestSectionsInErrorSkipsLinesInsideConfigSnippets(t *testing.T) {
	require.Empty(t, sectionsInError(failedCfg, []int{17}))
}

func TestSectionsInErrorSkipsSectionsNoResourceProduces(t *testing.T) {
	require.Empty(t, sectionsInError(failedCfg, []int{26}))
}

func TestSectionsInErrorReportsEachSectionOnce(t *testing.T) {
	require.Equal(t, []Section{
		{Kind: SectionBackend, Name: "appNs_svc_first_https"},
		{Kind: SectionBackend, Name: "appNs_svc_second_https"},
	}, sectionsInError(failedCfg, []int{19, 14, 23}))
}

func TestSectionsInErrorIgnoresLinesBeyondTheFile(t *testing.T) {
	require.Empty(t, sectionsInError(failedCfg, []int{0, 999}))
}

func TestSectionsInErrorReadsTheRejectedFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "haproxy.cfg"), []byte(failedCfg), 0o600))
	configErr := errors.New("config : parsing [/etc/haproxy/haproxy.cfg:14] : errorfile : error opening file '/nonexistent'.\n" +
		"config : Error(s) found in configuration file : /etc/haproxy/haproxy.cfg")

	sections, err := SectionsInError(configErr, dir)

	require.NoError(t, err)
	require.Equal(t, []Section{{Kind: SectionBackend, Name: "appNs_svc_first_https"}}, sections)
}

func TestSectionsInErrorWithoutErrorReportsNothing(t *testing.T) {
	sections, err := SectionsInError(nil, t.TempDir())
	require.NoError(t, err)
	require.Empty(t, sections)
}

func TestSectionKeyIdentifiesASection(t *testing.T) {
	require.Equal(t, "global", Section{Kind: SectionGlobal}.Key())
	require.Equal(t, "backend/app", Section{Kind: SectionBackend, Name: "app"}.Key())
}
