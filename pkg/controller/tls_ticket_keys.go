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

package controller

import (
	"crypto/sha256"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
)

// tlsTicketKeysRing is the number of keys HAProxy keeps per tls-ticket-keys file
// (TLS_TICKETS_NO build option, 3 by default).
const tlsTicketKeysRing = 3

// tlsTicketKeysManager pushes a rotated tls-ticket-keys file to HAProxy through the
// runtime API, so that a rotation needs no reload. It runs on every sync tick and
// only talks to HAProxy when the file content changed. It falls back to a reload,
// which reads the file again, when the runtime update fails.
func (c *HAProxyController) tlsTicketKeysManager() {
	keysFile := c.store.ConfigMaps.Main.Annotations["tls-ticket-keys"]
	if keysFile == "" {
		c.tlsTicketKeysSum = [32]byte{}
		return
	}
	content, err := os.ReadFile(keysFile)
	if err != nil {
		return
	}
	sum := sha256.Sum256(content)
	if sum == c.tlsTicketKeysSum {
		return
	}
	desired := tlsTicketKeysFromFile(string(content))
	if len(desired) != tlsTicketKeysRing {
		logger.Errorf("tls-ticket-keys: %q holds fewer than %d keys, not applied", keysFile, tlsTicketKeysRing)
		return
	}

	dump, err := c.haproxy.ExecuteRaw("show tls-keys " + keysFile)
	if err != nil {
		logger.Errorf("tls-ticket-keys: cannot read the loaded keys: %s", err)
		instance.Reload("tls-ticket-keys file changed")
		c.tlsTicketKeysSum = sum
		return
	}
	if strings.Contains(dump, "unable to locate") {
		// The file is not set on any bind (yet): the next reload loads it.
		c.tlsTicketKeysSum = sum
		return
	}
	for _, key := range tlsTicketKeysToPush(tlsTicketKeysFromDump(dump), desired) {
		res, errSet := c.haproxy.ExecuteRaw(fmt.Sprintf("set ssl tls-key %s %s", keysFile, key))
		if errSet == nil && !strings.Contains(res, "updated") {
			errSet = fmt.Errorf("%s", strings.TrimSpace(res))
		}
		if errSet != nil {
			logger.Errorf("tls-ticket-keys: runtime update failed, reloading: %s", errSet)
			instance.Reload("tls-ticket-keys file changed")
			break
		}
		logger.Infof("tls-ticket-keys: pushed a rotated key for %q", keysFile)
	}
	c.tlsTicketKeysSum = sum
}

// tlsTicketKeysFromFile returns the keys HAProxy loads from a keys file: its last
// tlsTicketKeysRing non-empty lines, oldest first.
func tlsTicketKeysFromFile(content string) []string {
	var keys []string
	for _, line := range strings.Split(content, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			keys = append(keys, line)
		}
	}
	if len(keys) > tlsTicketKeysRing {
		keys = keys[len(keys)-tlsTicketKeysRing:]
	}
	return keys
}

// tlsTicketKeysFromDump parses "show tls-keys <file>", whose "<id>.<n> <key>" lines
// list the loaded keys oldest first, the encryption key being the second to last.
func tlsTicketKeysFromDump(dump string) []string {
	var keys []string
	for _, line := range strings.Split(dump, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && !strings.HasPrefix(fields[0], "#") && strings.Contains(fields[0], ".") {
			keys = append(keys, fields[1])
		}
	}
	return keys
}

// tlsTicketKeysToPush returns the keys to push, in order, to turn the loaded ring into
// the desired one. Each push appends a key and drops the oldest, so the keys already
// at the head of the desired ring are kept: a rotation that appended one key pushes
// only that key, and one already applied (by a reload in between) pushes nothing.
func tlsTicketKeysToPush(loaded, desired []string) []string {
	for kept := min(len(loaded), len(desired)); kept > 0; kept-- {
		if slices.Equal(loaded[len(loaded)-kept:], desired[:kept]) {
			return desired[kept:]
		}
	}
	return desired
}
