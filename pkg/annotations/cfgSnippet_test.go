// Copyright 2023 HAProxy Technologies LLC
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
	"reflect"
	"testing"
)

func Test_DisableConfigSnippets(t *testing.T) {
	tests := []struct {
		name                  string
		disableConfigSnippets string
		want                  map[cfgSnippetType]struct{}
	}{
		{
			name:                  "No meaningful value",
			disableConfigSnippets: "invalid",
			want:                  map[cfgSnippetType]struct{}{},
		},
		{
			name:                  "all",
			disableConfigSnippets: "all",
			want: map[cfgSnippetType]struct{}{
				configSnippetBackend:  {},
				configSnippetFrontend: {},
				configSnippetGlobal:   {},
			},
		},
		{
			name:                  "frontend only",
			disableConfigSnippets: "frontend",
			want: map[cfgSnippetType]struct{}{
				configSnippetFrontend: {},
			},
		},
		{
			name:                  "backend only",
			disableConfigSnippets: "backend",
			want: map[cfgSnippetType]struct{}{
				configSnippetBackend: {},
			},
		},
		{
			name:                  "global only",
			disableConfigSnippets: "global",
			want: map[cfgSnippetType]struct{}{
				configSnippetGlobal: {},
			},
		},
		{
			name:                  "frontend and backend",
			disableConfigSnippets: "backend,frontend",
			want: map[cfgSnippetType]struct{}{
				configSnippetFrontend: {},
				configSnippetBackend:  {},
			},
		},
		{
			name:                  "frontend and global, whitespaces",
			disableConfigSnippets: " frontend, global",
			want: map[cfgSnippetType]struct{}{
				configSnippetGlobal:   {},
				configSnippetFrontend: {},
			},
		},
		{
			name:                  "frontend global, backend",
			disableConfigSnippets: "backend,global,frontend",
			want: map[cfgSnippetType]struct{}{
				configSnippetBackend:  {},
				configSnippetFrontend: {},
				configSnippetGlobal:   {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DisableConfigSnippets(tt.disableConfigSnippets)
			if !reflect.DeepEqual(cfgSnippet.disabledSnippets, tt.want) {
				t.Errorf("DisabledConfigSnippets() = %v, want %v", cfgSnippet.disabledSnippets, tt.want)
			}
		})
	}
}
