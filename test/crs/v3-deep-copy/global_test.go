// Copyright 2019 HAProxy Technologies
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package v3

import (
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
)

func TestGlobalSpec_DeepCopyInto(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "test GlobalSpec DeepCopy",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = gofakeit.Seed(0)

			for range 50 {
				global := &v3.GlobalSpec{}
				_ = gofakeit.Struct(global)

				deepCopied := &v3.GlobalSpec{}
				global.DeepCopyInto(deepCopied)

				// Sort functions for map[string]XXX
				sortMaps := cmpopts.SortMaps(func(a, b string) bool {
					return a < b
				})

				if !cmp.Equal(deepCopied, global, sortMaps) {
					diff := cmp.Diff(deepCopied, global, sortMaps)
					t.Errorf("[%s] DeepCopy GlobalSpec does not match expectation\n%v", tt.name, diff)
				}
			}
		})
	}
}
