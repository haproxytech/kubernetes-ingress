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

package handler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/stretchr/testify/require"
)

func sslBind(name string, ssl bool, keys string) *models.Bind {
	return &models.Bind{Name: name, BindParams: models.BindParams{Ssl: ssl, TLSTicketKeys: keys}}
}

func TestTLSTicketKeysChanges(t *testing.T) {
	const keys = "/etc/haproxy/tls-ticket-keys/tls-ticket-keys"

	t.Run("set on the owned ssl binds only", func(t *testing.T) {
		binds := models.Binds{
			sslBind(BIND_IP_V4, true, ""),
			sslBind(BIND_IP_V6, true, ""),
			sslBind("quicv4", true, ""),
		}
		changed := tlsTicketKeysChanges(binds, keys)
		require.Len(t, changed, 2)
		require.Equal(t, keys, binds[0].TLSTicketKeys)
		require.Equal(t, keys, binds[1].TLSTicketKeys)
		require.Empty(t, binds[2].TLSTicketKeys, "quic binds are not owned by the https handler")
	})

	t.Run("steady state changes nothing", func(t *testing.T) {
		binds := models.Binds{sslBind(BIND_IP_V4, true, keys)}
		require.Empty(t, tlsTicketKeysChanges(binds, keys))
	})

	t.Run("removed when unset", func(t *testing.T) {
		binds := models.Binds{sslBind(BIND_IP_V4, true, keys)}
		require.Len(t, tlsTicketKeysChanges(binds, ""), 1)
		require.Empty(t, binds[0].TLSTicketKeys)
	})

	t.Run("cleared on a bind without ssl", func(t *testing.T) {
		binds := models.Binds{sslBind(BIND_UNIX_SOCKET, false, keys)}
		require.Len(t, tlsTicketKeysChanges(binds, keys), 1)
		require.Empty(t, binds[0].TLSTicketKeys)
	})
}

func TestResolveTLSTicketKeys(t *testing.T) {
	h := &HTTPS{}
	require.Empty(t, h.resolveTLSTicketKeys(""))
	require.Empty(t, h.resolveTLSTicketKeys(filepath.Join(t.TempDir(), "missing")),
		"an unreadable file must never reach the configuration")

	f := filepath.Join(t.TempDir(), "keys")
	require.NoError(t, os.WriteFile(f, []byte("k\n"), 0o600))
	require.Equal(t, f, h.resolveTLSTicketKeys(f))
}
