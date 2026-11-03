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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/fs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// slowCommit is a runtime API whose "commit ssl cert" takes a while, like a busy
// HAProxy renewing many certificates at once.
type slowCommit struct {
	recorder
	delay time.Duration
}

func (s *slowCommit) CertEntryCommit(filename string) error {
	time.Sleep(s.delay)
	return s.recorder.CertEntryCommit(filename)
}

// TestCertsUpdatedWaitsForWriters checks that the reload decision sees a certificate
// whose runtime update is still running. The runtime update runs in a writer goroutine,
// and the HTTPS handler asks CertsUpdated right after adding the secrets, before the
// controller waits for the writers. Answering false there lost the reload, and the
// certificate stayed stale on disk.
func TestCertsUpdatedWaitsForWriters(t *testing.T) {
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
	repository.SetAPI(&slowCommit{delay: 50 * time.Millisecond})

	_, err = repository.AddSecret(&store.Secret{
		Namespace: "web",
		Name:      "app-tls",
		Data: map[string][]byte{
			"tls.crt": []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----"),
			"tls.key": []byte("-----BEGIN PRIVATE KEY-----\nMIIB\n-----END PRIVATE KEY-----"),
		},
		Status: store.ADDED,
	}, certs.FT_CERT)
	require.NoError(t, err)
	t.Cleanup(fs.Writer.WaitUntilWritesDone)

	require.True(t, repository.CertsUpdated(), "a certificate being updated must request a reload")
}
