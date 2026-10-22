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

package instance

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// Map and certificate files are refreshed by parallel goroutines, each marking
// the configuration as needing a reload while the sync loop reads the flag.
// This test is a no-op without -race and a failure with it before the fix.
func TestReloadFlagIsConcurrencySafe(t *testing.T) {
	cmi := NewConfigurationManager()

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			cmi.Reset()
			cmi.SetReload("test reason")
			cmi.SetReloadIf(true, "test reason")
			_ = cmi.NeedReload()
		})
	}
	wg.Wait()

	// Every goroutine ends by setting the flag, so it must be set.
	require.True(t, cmi.NeedReload())
	cmi.Reset()
	require.False(t, cmi.NeedReload())
}
