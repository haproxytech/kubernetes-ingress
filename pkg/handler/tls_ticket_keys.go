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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

const (
	// tlsTicketKeysAnnotation names the Secret holding the keys, and the key of the
	// Secret data holding the keys file.
	tlsTicketKeysAnnotation = "tls-ticket-keys"
	// tlsTicketKeysRing is the number of keys HAProxy keeps per tls-ticket-keys file
	// (TLS_TICKETS_NO build option, 3 by default).
	tlsTicketKeysRing = 3
)

// syncTLSTicketKeys writes the keys of the tls-ticket-keys Secret to a file and sets
// it on every ssl bind of the https frontend, so that all instances sharing the
// Secret resume each other's sessions.
func (handler *HTTPS) syncTLSTicketKeys(k store.K8s, h haproxy.HAProxy) error {
	keysFile := filepath.Join(h.Env.CfgDir, tlsTicketKeysAnnotation)
	content, err := tlsTicketKeysFromSecret(k)
	switch {
	case err != nil:
		// An invalid Secret keeps the last valid keys in use, if any.
		logger.Errorf("tls-ticket-keys: %s", err)
		if _, errStat := os.Stat(keysFile); errStat != nil {
			keysFile = ""
		}
	case content == nil:
		keysFile = ""
	default:
		changed, errWrite := writeTLSTicketKeys(keysFile, content)
		if errWrite != nil {
			return fmt.Errorf("tls-ticket-keys: %w", errWrite)
		}
		instance.ReloadIf(changed, "tls-ticket-keys updated")
	}

	binds, err := h.FrontendBindsGet(h.FrontHTTPS)
	if err != nil {
		return fmt.Errorf("tls-ticket-keys: %w", err)
	}
	changed := tlsTicketKeysChanges(binds, keysFile)
	for _, bind := range changed {
		if err = h.FrontendBindEdit(h.FrontHTTPS, *bind); err != nil {
			return fmt.Errorf("tls-ticket-keys: %w", err)
		}
	}
	instance.ReloadIf(len(changed) > 0, "tls-ticket-keys updated")
	if keysFile == "" {
		// Off the binds now: the next reload no longer reads it.
		_ = os.Remove(filepath.Join(h.Env.CfgDir, tlsTicketKeysAnnotation))
	}
	return nil
}

// tlsTicketKeysFromSecret returns the keys file held by the tls-ticket-keys Secret,
// nil when the annotation is unset, or an error when HAProxy would refuse the file.
func tlsTicketKeysFromSecret(k store.K8s) ([]byte, error) {
	secret, err := annotations.Secret(tlsTicketKeysAnnotation, k.ConfigMaps.Main.Namespace, k, k.ConfigMaps.Main.Annotations)
	if err != nil || secret == nil {
		return nil, err
	}
	content, ok := secret.Data[tlsTicketKeysAnnotation]
	if !ok {
		return nil, fmt.Errorf("secret '%s/%s' has no %q key", secret.Namespace, secret.Name, tlsTicketKeysAnnotation)
	}
	if _, err = parseTLSTicketKeys(string(content)); err != nil {
		return nil, fmt.Errorf("secret '%s/%s': %w", secret.Namespace, secret.Name, err)
	}
	return content, nil
}

// parseTLSTicketKeys checks a keys file with the rules HAProxy loads it with, since
// it refuses the whole configuration on a bad file, and returns the keys HAProxy
// keeps: the last tlsTicketKeysRing ones, oldest first.
func parseTLSTicketKeys(content string) ([]string, error) {
	var lines []string
	if content != "" {
		lines = strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	}
	if len(lines) < tlsTicketKeysRing {
		return nil, fmt.Errorf("%d keys, at least %d expected", len(lines), tlsTicketKeysRing)
	}
	size := 0
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
		key, err := base64.StdEncoding.DecodeString(lines[i])
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid base64", i+1)
		}
		// 48 bytes for AES-128 tickets, 80 for AES-256, the same for every key.
		if (len(key) != 48 && len(key) != 80) || (size != 0 && len(key) != size) {
			return nil, fmt.Errorf("line %d: wrong sized key", i+1)
		}
		size = len(key)
	}
	return lines[len(lines)-tlsTicketKeysRing:], nil
}

// writeTLSTicketKeys writes content to keysFile unless it already holds it, and
// reports whether it changed. The rename keeps a reload from reading a partial file.
func writeTLSTicketKeys(keysFile string, content []byte) (bool, error) {
	current, err := os.ReadFile(keysFile)
	if err == nil && bytes.Equal(current, content) {
		return false, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	tmp := keysFile + ".tmp"
	if err = os.WriteFile(tmp, content, 0o600); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, keysFile)
}

// tlsTicketKeysChanges sets keysFile on the ssl binds this handler owns (the quic
// ones are managed elsewhere), clears it on the others, and returns the binds it changed.
func tlsTicketKeysChanges(binds models.Binds, keysFile string) (changed models.Binds) {
	for _, bind := range binds {
		if bind.Name != BIND_IP_V4 && bind.Name != BIND_IP_V6 && bind.Name != BIND_UNIX_SOCKET {
			continue
		}
		want := ""
		if bind.Ssl {
			want = keysFile
		}
		if bind.TLSTicketKeys == want {
			continue
		}
		bind.TLSTicketKeys = want
		changed = append(changed, bind)
	}
	return changed
}
