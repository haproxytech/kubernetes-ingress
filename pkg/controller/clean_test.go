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

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// TestFailedSyncKeepsWhatItHasNotDone is why the event flags are consumed by the store cleanup
// and not by the handler which acted on them: a failed sync skips the cleanup entirely, so what
// it was asked to do is still pending on the next one.
//
// A status manager resetting the request itself would have dropped it on a transaction the
// commit refused, and nothing would ask again - a publish service event does not repeat.
func TestFailedSyncKeepsWhatItHasNotDone(t *testing.T) {
	c := controllerWithClass(t)
	ns := c.store.GetNamespace("ns")
	ing := registerRoute(ns, "app-ing", "example.ingress", "/", store.PATH_TYPE_PREFIX, "app")
	ing.ClassUpdated = true
	c.store.UpdateAllIngresses = true

	c.clean(true)

	require.True(t, c.store.UpdateAllIngresses, "a failed sync has published nothing, the request stands")
	require.True(t, ing.ClassUpdated)
	require.Equal(t, store.ADDED, ing.Status)

	c.clean(false)

	require.False(t, c.store.UpdateAllIngresses, "a completed sync has consumed it")
	require.False(t, ing.ClassUpdated)
	require.Equal(t, store.EMPTY, ing.Status)
}
