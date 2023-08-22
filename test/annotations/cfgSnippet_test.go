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

package annotations_test

import (
	"testing"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
)

func Test_DisableConfigSnippets(t *testing.T) {
	tests := []struct {
		name                  string
		disableConfigSnippets string
		want                  map[annotations.CfgSnippetType]bool
	}{
		{
			name:                  "No meaningful value",
			disableConfigSnippets: "invalid",
			want: map[annotations.CfgSnippetType]bool{
				annotations.ConfigSnippetBackend:  false,
				annotations.ConfigSnippetFrontend: false,
				annotations.ConfigSnippetGlobal:   false,
			},
		},
		{
			name:                  "all",
			disableConfigSnippets: "all",
			want: map[annotations.CfgSnippetType]bool{
				annotations.ConfigSnippetBackend:  true,
				annotations.ConfigSnippetFrontend: true,
				annotations.ConfigSnippetGlobal:   true,
			},
		},
		{
			name:                  "frontend only",
			disableConfigSnippets: "frontend",
			want: map[annotations.CfgSnippetType]bool{
				annotations.ConfigSnippetFrontend: true,
				annotations.ConfigSnippetBackend:  false,
				annotations.ConfigSnippetGlobal:   false,
			},
		},
		{
			name:                  "backend only",
			disableConfigSnippets: "backend",
			want: map[annotations.CfgSnippetType]bool{
				annotations.ConfigSnippetBackend:  true,
				annotations.ConfigSnippetFrontend: false,
				annotations.ConfigSnippetGlobal:   false,
			},
		},
		{
			name:                  "global only",
			disableConfigSnippets: "global",
			want: map[annotations.CfgSnippetType]bool{
				annotations.ConfigSnippetGlobal:   true,
				annotations.ConfigSnippetFrontend: false,
				annotations.ConfigSnippetBackend:  false,
			},
		},
		{
			name:                  "frontend and backend",
			disableConfigSnippets: "backend,frontend",
			want: map[annotations.CfgSnippetType]bool{
				annotations.ConfigSnippetFrontend: true,
				annotations.ConfigSnippetBackend:  true,
				annotations.ConfigSnippetGlobal:   false,
			},
		},
		{
			name:                  "frontend and global, whitespaces",
			disableConfigSnippets: " frontend, global",
			want: map[annotations.CfgSnippetType]bool{
				annotations.ConfigSnippetGlobal:   true,
				annotations.ConfigSnippetFrontend: true,
				annotations.ConfigSnippetBackend:  false,
			},
		},
		{
			name:                  "frontend global, backend",
			disableConfigSnippets: "backend,global,frontend",
			want: map[annotations.CfgSnippetType]bool{
				annotations.ConfigSnippetBackend:  true,
				annotations.ConfigSnippetFrontend: true,
				annotations.ConfigSnippetGlobal:   true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations.DisableConfigSnippets(tt.disableConfigSnippets)
			for snippetType, wantDisabled := range tt.want {
				disabled := annotations.IsConfigSnippetDisabled(snippetType)
				if disabled != wantDisabled {
					t.Errorf("DisabledConfigSnippets() for type %s = %v, want %v", snippetType, disabled, wantDisabled)
				}
			}
		})
	}
}
