package ingress

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// refuseRegex installs a fake HAProxy that refuses rules holding the word REFUSED.
func refuseRegex(t *testing.T) {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "haproxy")
	script := "#!/bin/sh\nif grep -q REFUSED; then exit 1; fi\nexit 0\n"
	require.NoError(t, os.WriteFile(binary, []byte(script), 0o700)) //nolint:gosec
	t.Cleanup(func() { common.SetHAProxyBinary("") })
	common.SetHAProxyBinary(binary)
}

func TestCORSOriginRefusedRegex(t *testing.T) {
	refuseRegex(t)
	for value, wantErr := range map[string]bool{"^https://example\\.com$": false, "REFUSED(": true, "*": false} {
		var list rules.List
		cors := NewResSetCORS(&list)
		values := map[string]string{"cors-enable": "true", "cors-allow-origin": value}
		require.NoError(t, cors.NewAnnotation("cors-enable").Process(store.K8s{}, values))
		err := cors.NewAnnotation("cors-allow-origin").Process(store.K8s{}, values)
		if wantErr {
			require.Error(t, err, value)
			require.Len(t, list, 1)
			continue
		}
		require.NoError(t, err, value)
		require.Len(t, list, 2)
	}
}

func TestPathRewriteRefusedRegex(t *testing.T) {
	refuseRegex(t)
	process := func(value string) (rules.List, error) {
		var list rules.List
		err := NewReqPathRewrite("path-rewrite", &list).Process(store.K8s{}, map[string]string{"path-rewrite": value})
		return list, err
	}
	list, err := process(`^/foo/(.*)$ /\1`)
	require.NoError(t, err)
	require.Len(t, list, 1)

	list, err = process("REFUSED( /x")
	require.Error(t, err)
	require.Empty(t, list)

	// A single parameter is a replacement, not a regex.
	list, err = process("/REFUSED")
	require.NoError(t, err)
	require.Len(t, list, 1)

	// Earlier lines stay, as with any other invalid line.
	list, err = process("^/a /b\nREFUSED( /x")
	require.Error(t, err)
	require.Len(t, list, 1)
}
