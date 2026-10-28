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

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// frontendSnippetClient only implements the call made by UpdateFrontendCfgSnippet.
type frontendSnippetClient struct {
	api.HAProxyClient
	written map[string][]string
}

func (c *frontendSnippetClient) FrontendCfgSnippetSet(frontendName string, value []string) error {
	c.written[frontendName] = value
	return nil
}

func TestUpdateFrontendCfgSnippetReportsConfigMapChange(t *testing.T) {
	InitCfgSnippet()
	t.Cleanup(InitCfgSnippet)

	frontend := "http"
	snippet := NewCfgSnippet(ConfigSnippetOptions{Name: "frontend-config-snippet", Frontend: &frontend})
	client := &frontendSnippetClient{written: map[string][]string{}}

	for _, value := range []string{"http-request set-header X-Snippet v1", "http-request set-header X-Snippet v2"} {
		if err := snippet.Process(store.K8s{}, map[string]string{"frontend-config-snippet": value}); err != nil {
			t.Fatal(err)
		}
		updated, err := UpdateFrontendCfgSnippet(client, frontend)
		if err != nil {
			t.Fatal(err)
		}
		if len(updated) == 0 {
			t.Errorf("snippet changed to %q but no update reported, so no reload is triggered", value)
		}
		if got := client.written[frontend]; len(got) != 1 || got[0] != value {
			t.Errorf("written snippet = %q, want %q", got, value)
		}
	}

	updated, err := UpdateFrontendCfgSnippet(client, frontend)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 0 {
		t.Errorf("unchanged snippet reported as updated: %q", updated)
	}
}
