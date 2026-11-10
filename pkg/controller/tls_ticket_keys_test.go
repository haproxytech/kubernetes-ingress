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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTLSTicketKeysFromFile(t *testing.T) {
	require.Equal(t, []string{"b", "c", "d"}, tlsTicketKeysFromFile("a\nb\n\n c \nd\n"),
		"HAProxy loads the last keys of the file")
	require.Equal(t, []string{"a", "b"}, tlsTicketKeysFromFile("a\nb\n"))
}

func TestTLSTicketKeysFromDump(t *testing.T) {
	dump := "# id secret\n# 0 (/etc/haproxy/tls-ticket-keys/tls-ticket-keys)\n0.0 a\n0.1 b\n0.2 c\n\n"
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
