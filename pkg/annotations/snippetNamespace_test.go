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

	"github.com/stretchr/testify/assert"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func TestSnippetNamespace(t *testing.T) {
	ingress := &store.Ingress{IngressCore: store.IngressCore{Namespace: "ingress-ns", Name: "ing"}}
	service := &store.Service{Namespace: "service-ns", Name: "svc"}

	tests := []struct {
		name    string
		snippet *CfgSnippet
		want    string
	}{
		{name: "ingress namespace", snippet: &CfgSnippet{ingress: ingress, service: service}, want: "ingress-ns"},
		{name: "service namespace without ingress", snippet: &CfgSnippet{service: service}, want: "service-ns"},
		{name: "no namespace context", snippet: &CfgSnippet{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, snippetNamespace(tt.snippet))
		})
	}
}
