// Copyright 2026 HAProxy Technologies LLC
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

package api

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisposeTransactionAfterNativeCommitFailure(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "haproxy.cfg")
	transactionDir := filepath.Join(dir, "transactions")
	before := []byte("# _version=1\nglobal\n  maxconn 1234\n")
	require.NoError(t, os.WriteFile(configFile, before, 0o600))
	require.NoError(t, os.Mkdir(transactionDir, 0o700))
	// The failure below is from the native file writer, before validation.
	validator, err := exec.LookPath("echo")
	require.NoError(t, err)
	client, err := New(transactionDir, configFile, validator, "")
	require.NoError(t, err)
	native := client.(*clientNative)
	configuration, err := native.nativeAPI.Configuration()
	require.NoError(t, err)
	var transactionIDs []string
	for attempt := range 3 {
		require.NoError(t, client.APIStartTransaction())
		id := native.activeTransaction
		transactionIDs = append(transactionIDs, id)
		global, err := client.GlobalGetConfiguration()
		require.NoError(t, err)
		require.NotNil(t, global.PerformanceOptions)
		global.PerformanceOptions.Maxconn = int64(2345 + attempt)
		require.NoError(t, client.GlobalPushConfiguration(*global))
		// A directory at the exact transaction file path makes native Save fail
		// even for a privileged test runner. No client method is intercepted.
		blockedFile := filepath.Join(transactionDir, filepath.Base(configFile)+"."+id)
		require.NoFileExists(t, blockedFile, "the client uses nonpersistent transactions")
		require.NoError(t, os.Mkdir(blockedFile, 0o700))
		err = client.APIFinalCommitTransaction()
		require.ErrorContains(t, err, blockedFile)
		t.Logf("attempt %d native commit: %v", attempt+1, err)
		require.True(t, configuration.HasParser(id), "native failed commit retains its parser")
		require.NoError(t, os.Remove(blockedFile))
		client.APIDisposeTransaction()
		require.Empty(t, native.activeTransaction)
		retained := 0
		for _, id := range transactionIDs {
			if configuration.HasParser(id) {
				retained++
			}
		}
		t.Logf("attempt %d retained parsers after disposal: %d", attempt+1, retained)
		assert.Zero(t, retained, "abandoned native transactions must not accumulate")
		actual, err := os.ReadFile(configFile)
		require.NoError(t, err)
		require.Equal(t, before, actual)
	}

	require.NoError(t, client.APIStartTransaction())
	global, err := client.GlobalGetConfiguration()
	require.NoError(t, err)
	global.PerformanceOptions.Maxconn = 3456
	require.NoError(t, client.GlobalPushConfiguration(*global))
	require.NoError(t, client.APIFinalCommitTransaction())
	require.False(t, configuration.HasParser(native.activeTransaction))
	committed, err := os.ReadFile(configFile)
	require.NoError(t, err)
	require.Contains(t, string(committed), "maxconn 3456")
	client.APIDisposeTransaction()
	client.APIDisposeTransaction()
	global, err = client.GlobalGetConfiguration()
	require.NoError(t, err)
	require.EqualValues(t, 3456, global.PerformanceOptions.Maxconn, "disposal must not remove the committed parser")
	// An unchanged transaction is already deleted by final commit.
	require.NoError(t, client.APIStartTransaction())
	require.NoError(t, client.APIFinalCommitTransaction())
	client.APIDisposeTransaction()
	actual, err := os.ReadFile(configFile)
	require.NoError(t, err)
	require.Equal(t, committed, actual, "disposal must not change the committed file")
	transactions, err := configuration.GetTransactions("in_progress")
	require.NoError(t, err)
	assert.Empty(t, *transactions)
}
