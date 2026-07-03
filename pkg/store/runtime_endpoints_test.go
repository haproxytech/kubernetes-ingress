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

package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRuntimeEndpointsSorted verifies that Sorted() returns the endpoints ordered
// deterministically by address then port, regardless of the (random) map
// iteration order. This is what guarantees a stable address-to-server-slot
// assignment across controller instances.
func TestRuntimeEndpointsSorted(t *testing.T) {
	endpoints := RuntimeEndpoints{
		{Address: "10.0.0.3", Port: 80}:   {},
		{Address: "10.0.0.1", Port: 80}:   {},
		{Address: "10.0.0.2", Port: 8080}: {},
		{Address: "10.0.0.2", Port: 80}:   {},
	}
	want := []RuntimeEndpoint{
		{Address: "10.0.0.1", Port: 80},
		{Address: "10.0.0.2", Port: 80},
		{Address: "10.0.0.2", Port: 8080},
		{Address: "10.0.0.3", Port: 80},
	}
	// Run several times to defeat the randomized map iteration order.
	for i := 0; i < 100; i++ {
		assert.Equal(t, want, endpoints.Sorted())
	}
}

// TestRuntimeEndpointsSortedEmpty verifies Sorted() copes with an empty set.
func TestRuntimeEndpointsSortedEmpty(t *testing.T) {
	assert.Empty(t, RuntimeEndpoints{}.Sorted())
}
