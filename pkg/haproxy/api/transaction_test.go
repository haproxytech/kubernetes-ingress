package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	clientnative "github.com/haproxytech/client-native/v6"
	"github.com/haproxytech/client-native/v6/configuration"
	cfgoptions "github.com/haproxytech/client-native/v6/configuration/options"
	"github.com/haproxytech/client-native/v6/options"
	"github.com/stretchr/testify/require"
)

// newTransactionTestClient wires a real configuration client to a fake haproxy binary.
func newTransactionTestClient(t *testing.T) (*clientNative, configuration.Configuration) {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "haproxy")
	script := "#!/bin/sh\ncase \"$*\" in *-v*) echo 'HAProxy version 3.2.0 2025/01/01';; esac\nexit 0\n"
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o700))
	cfg := filepath.Join(root, "haproxy.cfg")
	require.NoError(t, os.WriteFile(cfg, []byte("# _version=1\nglobal\n\ndefaults unnamed_defaults_1\n"), 0o600))
	conf, err := configuration.New(context.Background(),
		cfgoptions.ConfigurationFile(cfg),
		cfgoptions.HAProxyBin(bin),
		cfgoptions.TransactionsDir(filepath.Join(root, "transactions")),
	)
	require.NoError(t, err)
	native, err := clientnative.New(context.Background(), options.Configuration(conf))
	require.NoError(t, err)
	return &clientNative{nativeAPI: native}, conf
}

func inProgressTransactions(t *testing.T, conf configuration.Configuration) int {
	t.Helper()
	ts, err := conf.GetTransactions("in_progress")
	require.NoError(t, err)
	return len(*ts)
}

// A sync that fails before commit must not leave its transaction in memory.
func TestDisposeReleasesUncommittedTransaction(t *testing.T) {
	c, conf := newTransactionTestClient(t)
	require.NoError(t, c.APIStartTransaction())
	require.Equal(t, 1, inProgressTransactions(t, conf))

	c.APIDisposeTransaction()

	require.Equal(t, 0, inProgressTransactions(t, conf))
}

func TestDisposeAfterCommit(t *testing.T) {
	c, conf := newTransactionTestClient(t)
	require.NoError(t, c.APIStartTransaction())
	require.NoError(t, c.APICommitTransaction())

	c.APIDisposeTransaction()

	require.Equal(t, 0, inProgressTransactions(t, conf))
	require.Empty(t, c.activeTransaction)
}
