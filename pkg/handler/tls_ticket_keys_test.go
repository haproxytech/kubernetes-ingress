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

package handler

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func ticketKey(b byte, size int) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, size))
}

func validTicketKeys() string {
	return ticketKey('a', 80) + "\n" + ticketKey('b', 80) + "\n" + ticketKey('c', 80) + "\n"
}

func sslBind(name string, ssl bool, keys string) *models.Bind {
	return &models.Bind{Name: name, BindParams: models.BindParams{Ssl: ssl, TLSTicketKeys: keys}}
}

func TestTLSTicketKeysChanges(t *testing.T) {
	const keys = "/etc/haproxy/tls-ticket-keys"

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

func TestParseTLSTicketKeys(t *testing.T) {
	a, b, c, d := ticketKey('a', 80), ticketKey('b', 80), ticketKey('c', 80), ticketKey('d', 80)

	keys, err := parseTLSTicketKeys(a + "\n" + b + "\r\n" + c + "\n" + d)
	require.NoError(t, err)
	require.Equal(t, []string{b, c, d}, keys, "HAProxy keeps the last keys of the file")

	keys, err = parseTLSTicketKeys(ticketKey('a', 48) + "\n" + ticketKey('b', 48) + "\n" + ticketKey('c', 48) + "\n")
	require.NoError(t, err)
	require.Len(t, keys, tlsTicketKeysRing)

	// Every file HAProxy refuses to load.
	for name, content := range map[string]string{
		"empty":         "",
		"two keys":      a + "\n" + b + "\n",
		"blank line":    a + "\n\n" + b + "\n" + c + "\n",
		"trailing line": a + "\n" + b + "\n" + c + "\n\n",
		"not base64":    a + "\n" + b + "\nnot-base64!\n",
		"wrong size":    a + "\n" + b + "\n" + ticketKey('c', 32) + "\n",
		"mixed sizes":   a + "\n" + b + "\n" + ticketKey('c', 48) + "\n",
		"padded line":   a + "\n " + b + "\n" + c + "\n",
	} {
		_, err := parseTLSTicketKeys(content)
		require.Error(t, err, name)
	}
}

func TestTLSTicketKeysFromSecret(t *testing.T) {
	k := func(annotation string, data map[string][]byte) store.K8s {
		return store.K8s{
			ConfigMaps: store.ConfigMaps{Main: &store.ConfigMap{
				Namespace:   "haproxy-controller",
				Annotations: map[string]string{"tls-ticket-keys": annotation},
			}},
			Namespaces: map[string]*store.Namespace{"haproxy-controller": {Secret: map[string]*store.Secret{
				"keys": {Namespace: "haproxy-controller", Name: "keys", Data: data, Status: store.ADDED},
			}}},
		}
	}
	valid := map[string][]byte{"tls-ticket-keys": []byte(validTicketKeys())}

	content, err := tlsTicketKeysFromSecret(k("", valid))
	require.NoError(t, err)
	require.Nil(t, content, "unset annotation")

	for _, annotation := range []string{"keys", "haproxy-controller/keys"} {
		content, err = tlsTicketKeysFromSecret(k(annotation, valid))
		require.NoError(t, err, "the namespace defaults to the ConfigMap one")
		require.Equal(t, validTicketKeys(), string(content))
	}

	_, err = tlsTicketKeysFromSecret(k("missing", valid))
	require.Error(t, err)
	_, err = tlsTicketKeysFromSecret(k("keys", map[string][]byte{"other": []byte(validTicketKeys())}))
	require.Error(t, err, "no tls-ticket-keys key in the Secret")
	_, err = tlsTicketKeysFromSecret(k("keys", map[string][]byte{"tls-ticket-keys": []byte("k\n")}))
	require.Error(t, err, "a file HAProxy refuses must never reach the configuration")
}

func TestWriteTLSTicketKeys(t *testing.T) {
	f := filepath.Join(t.TempDir(), "tls-ticket-keys")

	changed, err := writeTLSTicketKeys(f, []byte("a"))
	require.NoError(t, err)
	require.True(t, changed)
	changed, err = writeTLSTicketKeys(f, []byte("a"))
	require.NoError(t, err)
	require.False(t, changed, "same content, no reload")
	changed, err = writeTLSTicketKeys(f, []byte("b"))
	require.NoError(t, err)
	require.True(t, changed)

	info, err := os.Stat(f)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestTLSTicketKeysLoaded(t *testing.T) {
	const keysFile = "/etc/haproxy/tls-ticket-keys"
	list := "# id (file)\n0 (" + keysFile + ")\n1 (/etc/haproxy/other)\n\n"
	require.True(t, tlsTicketKeysLoaded(list, keysFile))
	require.False(t, tlsTicketKeysLoaded(list, "/etc/haproxy"))
	require.False(t, tlsTicketKeysLoaded("# id (file)\n", keysFile), "no keys file set on the binds")
}

func TestTLSTicketKeysFromDump(t *testing.T) {
	dump := "# id secret\n# 0 (/etc/haproxy/tls-ticket-keys)\n0.0 a\n0.1 b\n0.2 c\n\n"
	require.Equal(t, []string{"a", "b", "c"}, tlsTicketKeysFromDump(dump))
}

func TestTLSTicketKeysToPush(t *testing.T) {
	loaded := []string{"a", "b", "c"}
	tests := []struct {
		name    string
		desired []string
		want    []string
	}{
		{"unchanged", []string{"a", "b", "c"}, []string{}},
		{"one key rotated", []string{"b", "c", "d"}, []string{"d"}},
		{"two keys rotated", []string{"c", "d", "e"}, []string{"d", "e"}},
		{"all keys replaced", []string{"x", "y", "z"}, []string{"x", "y", "z"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tlsTicketKeysToPush(loaded, tt.desired)
			require.ElementsMatch(t, tt.want, got)
			// Replaying the pushes on the ring must give the desired ring.
			ring := append([]string{}, loaded...)
			for _, k := range got {
				ring = append(ring[1:], k)
			}
			require.Equal(t, tt.desired, ring)
		})
	}
}
