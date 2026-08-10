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

package annotations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBackendNamesCoversWhatIsEasyToMiss pins the four names a naive implementation would
// leave out. No count is asserted: adding an annotation to the registry must not break this
// test, that being the point of deriving the list from it.
func TestBackendNamesCoversWhatIsEasyToMiss(t *testing.T) {
	names := BackendNames()

	// Only returned by Backend() for a backend already in http mode.
	require.Contains(t, names, "check-http")
	require.Contains(t, names, "forwarded-for")
	// Not in the registry at all: applied by service.HandleBackend and getBackendModel.
	require.Contains(t, names, "backend-config-snippet")
	require.Contains(t, names, "cr-backend")
	// A plain registry entry, to catch a list which would have lost its base.
	require.Contains(t, names, "load-balance")
}

// TestBackendNamesExcludesWhatIsNotBackendScoped keeps the list from drifting into a
// catch-all: a frontend rule annotation is not something an ingress loses to the owner of
// a backend, and cookie-persistence is resolved from the service annotations only.
func TestBackendNamesExcludesWhatIsNotBackendScoped(t *testing.T) {
	names := BackendNames()

	require.NotContains(t, names, "allow-list")
	require.NotContains(t, names, "ssl-redirect")
	require.NotContains(t, names, "cookie-persistence")
	require.NotContains(t, names, "ssl-passthrough")
}

// TestBackendNamesIsStable matters for the message built from it: a list coming out of a
// map iteration would produce a different log line on every sync, which reads as a
// different event.
func TestBackendNamesIsStable(t *testing.T) {
	first := BackendNames()
	for range 10 {
		require.Equal(t, first, BackendNames())
	}
}
