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

package api

import (
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/stretchr/testify/require"
)

// TestPreviousBackendsRoundTripKeepsControllerState is the regression test for the
// backends state lost around a failed transaction: Backend embeds models.Backend,
// which brings its own MarshalJSON/UnmarshalJSON. Those promoted methods used to
// serialize only the model, so Push/Pop silently reset Permanent, Used and
// ConfigSnippets to their zero value. A permanent backend restored as
// non-permanent is then deleted by BackendDeleteAllUnnecessary even though the
// configuration still references it.
func TestPreviousBackendsRoundTripKeepsControllerState(t *testing.T) {
	client := &clientNative{
		backends: map[string]Backend{
			"ssl-backend": {
				Backend: models.Backend{
					BackendBase: models.BackendBase{Name: "ssl-backend", Mode: "tcp"},
					Servers: map[string]models.Server{
						"https": {Name: "https", Address: "unix@/run/ssl-frontend.sock"},
					},
				},
				ConfigSnippets: []string{"tcp-request inspect-delay 5s"},
				Permanent:      true,
				Used:           true,
			},
		},
	}

	require.NoError(t, client.PushPreviousBackends())
	// A failed sync throws away the current state and restores the snapshot.
	clear(client.backends)
	require.NoError(t, client.PopPreviousBackends())

	restored, ok := client.backends["ssl-backend"]
	require.True(t, ok, "restored snapshot must still contain the backend")
	require.True(t, restored.Permanent,
		"a permanent backend must stay permanent across a failed transaction, else it gets garbage-collected while still referenced")
	require.True(t, restored.Used, "the used flag must survive the snapshot round-trip")
	require.Equal(t, []string{"tcp-request inspect-delay 5s"}, restored.ConfigSnippets,
		"config snippets must survive the snapshot round-trip")
	// The embedded model must keep round-tripping as before.
	require.Equal(t, "tcp", restored.Mode)
	require.Contains(t, restored.Servers, "https")
}

// TestPopPreviousBackendsWithoutSnapshot checks the no-snapshot path: a failure
// happening before any successful sync must leave no stale backend behind.
func TestPopPreviousBackendsWithoutSnapshot(t *testing.T) {
	client := &clientNative{
		backends: map[string]Backend{
			"ssl-backend": {Backend: models.Backend{BackendBase: models.BackendBase{Name: "ssl-backend"}}},
		},
	}

	require.NoError(t, client.PopPreviousBackends())
	require.Empty(t, client.backends)
}
