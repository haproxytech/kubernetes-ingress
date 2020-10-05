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

package configsnippet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_genericAttribute_Overridden_Ok(t *testing.T) {
	tests := map[string]struct {
		configSnippet string
		attributeName string
	}{
		"config-snippet-empty": {
			configSnippet: "",
			attributeName: "not-exists",
		},
		"config-snippet-data": {
			configSnippet: "some random data",
			attributeName: "not-exists",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Nil(t, NewGenericAttribute(tt.attributeName).Overridden(tt.configSnippet))
		})
	}
}

func Test_genericAttribute_Overridden_Fail(t *testing.T) {
	tests := map[string]struct {
		configSnippet string
		attributeName string
	}{
		"cookie-persistence": {
			configSnippet: "cookie JSESSIONID prefix",
			attributeName: "cookie",
		},
		"forwarded-for": {
			configSnippet: "no option forwardfor",
			attributeName: "option forwardfor",
		},
		"load-balance": {
			configSnippet: "balance leastconn",
			attributeName: "balance",
		},
		"check-http": {
			configSnippet: `option httpchk OPTIONS * HTTP/1.1\r\nHost:\ www`,
			attributeName: "option httpchk",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, NewGenericAttribute(tt.attributeName).Overridden(tt.configSnippet))
		})
	}
}
