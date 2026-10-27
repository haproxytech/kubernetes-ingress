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

package certs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/haproxytech/client-native/v6/runtime"

	"github.com/haproxytech/kubernetes-ingress/pkg/fs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/secret"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// recorder answers the runtime commands instead of sending them.
type recorder struct {
	api.HAProxyClient
	mu    sync.Mutex
	calls []string
}

func (r *recorder) add(format string, args ...any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, fmt.Sprintf(format, args...))
	return nil
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *recorder) CertEntryCreate(filename string) error { return r.add("new ssl cert %s", filename) }

func (r *recorder) CertEntrySet(filename string, _ []byte) error {
	return r.add("set ssl cert %s", filename)
}

func (r *recorder) CertEntryCommit(filename string) error {
	return r.add("commit ssl cert %s", filename)
}

func (r *recorder) CertEntryAbort(filename string) error { return r.add("abort ssl cert %s", filename) }

func (r *recorder) CertEntryDelete(filename string) error { return r.add("del ssl cert %s", filename) }

func (r *recorder) CrtListEntryAdd(crtList string, _ runtime.CrtListEntry) error {
	return r.add("add ssl crt-list %s", crtList)
}

// TestSharedSecretConverges is the regression test of the reload loop: a Secret used by an
// Ingress and by a TCP custom resource feeds two certificates, and a pass where nothing
// changed must neither delete one of them nor ask for a reload. Skipping the second one
// left its certificate marked as unused, which made the cleanup pass report a deletion and
// reload on every reconciliation.
func TestSharedSecretConverges(t *testing.T) {
	root := t.TempDir()
	dirs := map[string]string{}
	for _, name := range []string{"frontend", "backend", "ca", "tcp"} {
		dirs[name] = filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(dirs[name], 0o755))
	}
	repository, err := certs.New(certs.Env{
		MainDir:     root,
		FrontendDir: dirs["frontend"],
		BackendDir:  dirs["backend"],
		CaDir:       dirs["ca"],
		TCPCRDir:    dirs["tcp"],
	})
	require.NoError(t, err)
	commands := &recorder{}
	repository.SetAPI(commands)

	k := store.K8s{
		Namespaces: map[string]*store.Namespace{
			"web": {Secret: map[string]*store.Secret{
				"app-tls": {
					Namespace: "web",
					Name:      "app-tls",
					Data: map[string][]byte{
						"tls.crt": []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----"),
						"tls.key": []byte("-----BEGIN PRIVATE KEY-----\nMIIB\n-----END PRIVATE KEY-----"),
					},
					Status: store.EMPTY,
				},
			}},
		},
	}
	name := types.NamespacedName{Namespace: "web", Name: "app-tls"}

	// One reconciliation: the ingress loop writes the Ingress certificate, the TCP custom
	// resource handler writes the TCP one, the refresh pass walks the directories, and the
	// controller resets the "in use" marks once the pass is over.
	pass := func() {
		ingressPath, errAdd := secret.Add(k, repository, secret.Secret{
			Name:       name,
			OwnerType:  secret.OWNERTYPE_INGRESS,
			OwnerName:  "web/bridge",
			SecretType: certs.FT_CERT,
		})
		require.NoError(t, errAdd)
		tcpPath, errAdd := secret.Add(k, repository, secret.Secret{
			Name:       name,
			OwnerType:  secret.OWNERTYPE_TCP_CR,
			OwnerName:  "web/app-tcp",
			SecretType: certs.TCP_CERT,
		})
		require.NoError(t, errAdd)
		require.NotEqual(t, ingressPath, tcpPath, "one file per certificate kind")

		// The controller waits for the file writes before committing the
		// configuration, and so does this test before looking at them.
		fs.Writer.WaitUntilWritesDone()
		repository.RefreshCerts(commands)
		repository.CleanCerts()

		for _, path := range []string{ingressPath, tcpPath} {
			info, errStat := os.Stat(path)
			require.NoError(t, errStat, "every certificate must be on disk")
			require.NotZero(t, info.Size())
		}
	}

	instance.Reset()
	pass()
	require.False(t, instance.NeedReload(), "the first pass writes the certificates, it deletes none")

	// Nothing changed: the same two certificates are expected, no runtime command is sent
	// and no reload is asked for.
	written := commands.count()
	instance.Reset()
	pass()
	require.Equal(t, written, commands.count(), "a pass where nothing changed sends no runtime command")
	require.False(t, instance.NeedReload(), "a pass where nothing changed must not reload")
}
