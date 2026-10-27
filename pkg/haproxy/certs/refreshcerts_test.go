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

package certs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/haproxytech/client-native/v6/runtime"
	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/fs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
)

// fakeAPI records the runtime commands instead of sending them.
type fakeAPI struct {
	api.HAProxyClient
	mu          sync.Mutex
	calls       []string
	caInUse     bool
	commitCAErr error
}

func (f *fakeAPI) record(format string, args ...any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
	return nil
}

func (f *fakeAPI) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.calls...)
}

func (f *fakeAPI) CertEntryCreate(filename string) error {
	return f.record("new ssl cert %s", filename)
}

func (f *fakeAPI) CertEntrySet(filename string, _ []byte) error {
	return f.record("set ssl cert %s", filename)
}

func (f *fakeAPI) CertEntryCommit(filename string) error {
	return f.record("commit ssl cert %s", filename)
}

func (f *fakeAPI) CertEntryAbort(filename string) error {
	return f.record("abort ssl cert %s", filename)
}

func (f *fakeAPI) CertEntryDelete(filename string) error {
	return f.record("del ssl cert %s", filename)
}

func (f *fakeAPI) CertAuthEntryCreate(filename string) error {
	return f.record("new ssl ca-file %s", filename)
}

func (f *fakeAPI) CertAuthEntrySet(filename string, _ []byte) error {
	return f.record("set ssl ca-file %s", filename)
}

func (f *fakeAPI) CertAuthEntryCommit(filename string) error {
	if err := f.record("commit ssl ca-file %s", filename); err != nil {
		return err
	}
	return f.commitCAErr
}

func (f *fakeAPI) CertAuthEntryAbort(filename string) error {
	return f.record("abort ssl ca-file %s", filename)
}

func (f *fakeAPI) CertAuthEntryDelete(filename string) error {
	if err := f.record("del ssl ca-file %s", filename); err != nil {
		return err
	}
	if f.caInUse {
		return errors.New("CA file is in use, can't be deleted")
	}
	return nil
}

func (f *fakeAPI) CrtListEntryAdd(crtList string, entry runtime.CrtListEntry) error {
	return f.record("add ssl crt-list %s %s", crtList, entry.File)
}

func (f *fakeAPI) CrtListEntryDelete(crtList, filename string, _ *int64) error {
	return f.record("del ssl crt-list %s %s", crtList, filename)
}

// newTestRepository builds a certificate repository over temporary directories, its
// runtime commands recorded rather than sent.
func newTestRepository(t *testing.T) (*certs, *fakeAPI) {
	t.Helper()
	root := t.TempDir()
	dirs := map[string]string{}
	for _, name := range []string{"frontend", "backend", "ca", "tcp"} {
		dirs[name] = filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(dirs[name], 0o755))
	}
	repository, err := New(Env{
		MainDir:     root,
		FrontendDir: dirs["frontend"],
		BackendDir:  dirs["backend"],
		CaDir:       dirs["ca"],
		TCPCRDir:    dirs["tcp"],
	})
	require.NoError(t, err)
	c, ok := repository.(*certs)
	require.True(t, ok)

	commands := &fakeAPI{}
	c.SetAPI(commands)
	return c, commands
}

// TestRefreshCertsDeletesThroughTheCrtList covers the directory the generated
// configuration hands to "crt": HAProxy exposes it as a crt-list, which the runtime API
// can delete from.
func TestRefreshCertsDeletesThroughTheCrtList(t *testing.T) {
	c, commands := newTestRepository(t)
	instance.Reset()

	certPath := filepath.Join(env.FrontendDir, "web_app-tls.pem")
	require.NoError(t, os.WriteFile(certPath, []byte("pem"), 0o600))
	c.frontend["web_app-tls"] = &cert{name: "web/app-tls", path: certPath}

	c.refreshCerts(c.frontend, env.FrontendDir)

	require.Equal(t, []string{
		"del ssl crt-list " + env.FrontendDir + " " + certPath,
		"del ssl cert " + certPath,
	}, commands.recorded())
	require.False(t, instance.NeedReload(), "deleting through the crt-list needs no reload")
	require.NotContains(t, c.frontend, "web_app-tls")
}

// TestRefreshCertsLeavesTheDirectoriesWithoutCrtListAlone is the regression test for the
// spurious reload: the certificate of a TCP custom resource is referenced as a file, it
// belongs to no crt-list, so the runtime API cannot delete it and asking it to only
// reported a failure, which forced a reload on a pass where nothing had changed.
func TestRefreshCertsLeavesTheDirectoriesWithoutCrtListAlone(t *testing.T) {
	c, commands := newTestRepository(t)
	instance.Reset()

	certPath := filepath.Join(env.TCPCRDir, "web_app-tls.pem")
	require.NoError(t, os.WriteFile(certPath, []byte("pem"), 0o600))
	c.TCPCR["web_app-tls"] = &cert{name: "web/app-tls", path: certPath}

	c.refreshCerts(c.TCPCR, env.TCPCRDir)

	require.Empty(t, commands.recorded(), "no runtime command can address this certificate")
	require.False(t, instance.NeedReload(), "the pass must not reload for it")
	require.NotContains(t, c.TCPCR, "web_app-tls")

	// The file goes away once no configuration references it anymore.
	fs.RunDelayedFuncs()
	_, err := os.Stat(certPath)
	require.True(t, os.IsNotExist(err), "the unreferenced file is removed")
}

// TestUpdateRuntimeDoesNotAddToAMissingCrtList covers the other end: adding a new
// certificate to a crt-list which does not exist failed the whole runtime update, and the
// caller then requested a reload for a certificate that had just been committed.
func TestUpdateRuntimeDoesNotAddToAMissingCrtList(t *testing.T) {
	c, commands := newTestRepository(t)
	instance.Reset()

	certPath := filepath.Join(env.TCPCRDir, "web_app-tls.pem")
	updated, err := c.updateRuntime(certPath, []byte("pem"), false)
	require.NoError(t, err)
	require.True(t, updated, "the certificate was committed")

	require.Equal(t, []string{
		"new ssl cert " + certPath,
		"set ssl cert " + certPath,
		"commit ssl cert " + certPath,
	}, commands.recorded())
	require.False(t, instance.NeedReload(), "committing a certificate needs no reload")
}

// TestRefreshCertsDeletesCAFilesWithTheCACommands checks that a CA file is addressed by the
// CA file commands: the certificate store holds no CA, so the certificate commands cannot
// delete one.
func TestRefreshCertsDeletesCAFilesWithTheCACommands(t *testing.T) {
	c, commands := newTestRepository(t)
	instance.Reset()

	caPath := filepath.Join(env.CaDir, "web_client-ca.pem")
	require.NoError(t, os.WriteFile(caPath, []byte("pem"), 0o600))
	c.ca["web_client-ca"] = &cert{name: "web/client-ca", path: caPath, ca: true}

	c.refreshCerts(c.ca, env.CaDir)

	require.Equal(t, []string{"del ssl ca-file " + caPath}, commands.recorded())
	require.False(t, instance.NeedReload(), "the runtime API did the deletion, no reload is needed")
	require.NotContains(t, c.ca, "web_client-ca")
}

// TestRefreshCertsLeavesACAFileInUseToTheReload covers what the runtime API refuses: a CA
// file the configuration still names can only be dropped by the reload which loads a
// configuration without it, so the failure is expected and must not ask for a reload.
func TestRefreshCertsLeavesACAFileInUseToTheReload(t *testing.T) {
	c, commands := newTestRepository(t)
	commands.caInUse = true
	instance.Reset()

	caPath := filepath.Join(env.CaDir, "web_client-ca.pem")
	require.NoError(t, os.WriteFile(caPath, []byte("pem"), 0o600))
	c.ca["web_client-ca"] = &cert{name: "web/client-ca", path: caPath, ca: true}

	c.refreshCerts(c.ca, env.CaDir)

	require.Equal(t, []string{"del ssl ca-file " + caPath}, commands.recorded())
	require.False(t, instance.NeedReload(), "a CA file in use is expected, the reload handles it")

	fs.RunDelayedFuncs()
	_, err := os.Stat(caPath)
	require.True(t, os.IsNotExist(err), "the file is removed for the reload to come")
}

// TestUpdateRuntimeAbortsACAFileWithTheCACommand checks the transaction cleanup: aborting a
// CA file transaction with the certificate command would leave it half opened.
func TestUpdateRuntimeAbortsACAFileWithTheCACommand(t *testing.T) {
	c, commands := newTestRepository(t)
	instance.Reset()
	commands.commitCAErr = errors.New("commit failed")

	caPath := filepath.Join(env.CaDir, "web_client-ca.pem")
	_, err := c.updateRuntime(caPath, []byte("pem"), true)
	require.ErrorContains(t, err, "commit failed")

	require.Equal(t, []string{
		"new ssl ca-file " + caPath,
		"set ssl ca-file " + caPath,
		"commit ssl ca-file " + caPath,
		"abort ssl ca-file " + caPath,
	}, commands.recorded())
}
