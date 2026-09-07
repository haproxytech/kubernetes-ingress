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

	"github.com/haproxytech/client-native/v6/models"

	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func frontendSSLTestStore() store.K8s {
	return store.K8s{
		Namespaces: map[string]*store.Namespace{
			"default": {
				CRs: &store.CustomResources{
					Frontends: map[string]*v3.FrontendSpec{
						"test": {Frontend: models.Frontend{FrontendBase: models.FrontendBase{Name: "test"}}},
					},
				},
			},
		},
	}
}

func TestModelFrontendSSL(t *testing.T) {
	tests := []struct {
		name         string
		annotation   map[string]string
		expectedErr  string
		expectedCR   bool
		expectedMode string
	}{
		{
			name:        "annotation not set",
			annotation:  map[string]string{},
			expectedErr: "",
			expectedCR:  false,
		},
		{
			name:         "no suffix defaults to append",
			annotation:   map[string]string{"cr-frontend-ssl": "default/test"},
			expectedErr:  "",
			expectedCR:   true,
			expectedMode: "append",
		},
		{
			name:         "append suffix",
			annotation:   map[string]string{"cr-frontend-ssl": "default/test:append"},
			expectedErr:  "",
			expectedCR:   true,
			expectedMode: "append",
		},
		{
			name:         "prepend suffix",
			annotation:   map[string]string{"cr-frontend-ssl": "default/test:prepend"},
			expectedErr:  "",
			expectedCR:   true,
			expectedMode: "prepend",
		},
		{
			name:         "override suffix",
			annotation:   map[string]string{"cr-frontend-ssl": "default/test:override"},
			expectedErr:  "",
			expectedCR:   true,
			expectedMode: "override",
		},
		{
			name:        "invalid mode",
			annotation:  map[string]string{"cr-frontend-ssl": "default/test:replace"},
			expectedErr: "annotation 'cr-frontend-ssl': invalid lists merge mode 'replace', must be one of 'append', 'prepend' or 'override'",
		},
		{
			name:        "invalid path",
			annotation:  map[string]string{"cr-frontend-ssl": "/"},
			expectedErr: "annotation 'cr-frontend-ssl': invalid format",
		},
		{
			name:        "custom resource does not exist",
			annotation:  map[string]string{"cr-frontend-ssl": "default/unknown"},
			expectedErr: "annotation 'cr-frontend-ssl': custom resource 'default/unknown' does not exist",
		},
		{
			name:        "namespace does not exist",
			annotation:  map[string]string{"cr-frontend-ssl": "unknown/test"},
			expectedErr: "annotation 'cr-frontend-ssl': custom resource 'unknown/test' does not exist, namespace not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			frontend, mode, err := ModelFrontendSSL("cr-frontend-ssl", "", frontendSSLTestStore(), test.annotation)
			if test.expectedErr != "" {
				if err == nil || err.Error() != test.expectedErr {
					t.Fatalf("expected error '%s', got '%v'", test.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if test.expectedCR && frontend == nil {
				t.Fatal("expected a frontend model, got nil")
			}
			if !test.expectedCR && frontend != nil {
				t.Fatalf("expected no frontend model, got %v", frontend)
			}
			if mode != test.expectedMode {
				t.Fatalf("expected mode '%s', got '%s'", test.expectedMode, mode)
			}
		})
	}
}
