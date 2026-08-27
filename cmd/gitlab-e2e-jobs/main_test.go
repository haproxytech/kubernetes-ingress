// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvOrReturnsTheFallbackWhenUnset(t *testing.T) {
	t.Setenv("E2E_JOBS_TEST_KNOB", "")
	assert.Equal(t, "fallback", envOr("E2E_JOBS_TEST_KNOB", "fallback"))
}

func TestEnvOrPrefersTheEnvironment(t *testing.T) {
	t.Setenv("E2E_JOBS_TEST_KNOB", "chosen")
	assert.Equal(t, "chosen", envOr("E2E_JOBS_TEST_KNOB", "fallback"))
}
