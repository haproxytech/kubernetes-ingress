// Copyright 2026 HAProxy Technologies LLC
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
	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// frontendSnippetClient records the frontend config snippets pushed by UpdateFrontendCfgSnippet.
// The embedded interface satisfies the full api.HAProxyClient method set for compilation.
type frontendSnippetClient struct {
	api.HAProxyClient
	snippets map[string][]string
}

func (c *frontendSnippetClient) FrontendCfgSnippetSet(frontendName string, value []string) error {
	c.snippets[frontendName] = value
	return nil
}

// syncFrontendCfgSnippet mirrors HAProxyController.globalCfgSnipp: it processes the
// configmap annotations then pushes the frontend config snippets.
func syncFrontendCfgSnippet(t *testing.T, client *frontendSnippetClient, configmap map[string]string) []string {
	t.Helper()
	for _, a := range New().GlobalCfgSnipp() {
		require.NoError(t, a.Process(store.K8s{}, configmap))
	}
	updated, err := UpdateFrontendCfgSnippet(client, "http", "https", "stats")
	require.NoError(t, err)
	return updated
}

func TestUpdateFrontendCfgSnippet(t *testing.T) {
	InitCfgSnippet()
	t.Cleanup(InitCfgSnippet)
	client := &frontendSnippetClient{snippets: map[string][]string{}}

	updated := syncFrontendCfgSnippet(t, client, map[string]string{})
	assert.Empty(t, updated, "no snippet should not require a reload")

	updated = syncFrontendCfgSnippet(t, client, map[string]string{"frontend-config-snippet": "unique-id-header X-Unique-ID"})
	assert.NotEmpty(t, updated, "added snippet must require a reload")
	assert.Equal(t, []string{"unique-id-header X-Unique-ID"}, client.snippets["http"])

	updated = syncFrontendCfgSnippet(t, client, map[string]string{"frontend-config-snippet": "unique-id-header X-Request-ID"})
	assert.NotEmpty(t, updated, "changed snippet must require a reload")
	assert.Equal(t, []string{"unique-id-header X-Request-ID"}, client.snippets["http"])

	updated = syncFrontendCfgSnippet(t, client, map[string]string{"frontend-config-snippet": "unique-id-header X-Request-ID"})
	assert.Empty(t, updated, "unchanged snippet should not require a reload")
	assert.Equal(t, []string{"unique-id-header X-Request-ID"}, client.snippets["http"])

	updated = syncFrontendCfgSnippet(t, client, map[string]string{})
	assert.NotEmpty(t, updated, "removed snippet must require a reload")
	assert.Empty(t, client.snippets["http"], "removed snippet must be removed from the frontend")
}

func TestUpdateFrontendCfgSnippetWithCustomAnnotations(t *testing.T) {
	InitCfgSnippet()
	t.Cleanup(InitCfgSnippet)
	client := &frontendSnippetClient{snippets: map[string][]string{}}
	configmap := map[string]string{"frontend-config-snippet": "unique-id-header X-Unique-ID"}

	syncFrontendCfgSnippet(t, client, configmap)
	processConfigSnippetFrontendCustom("http", "custom-header", []string{"### custom-header ###", "http-request set-header X-Custom 1"}, 1)
	updated, err := UpdateFrontendCfgSnippet(client, "http")
	require.NoError(t, err)
	assert.NotEmpty(t, updated, "added custom annotation must require a reload")
	assert.Equal(t, []string{
		"### custom-header ###", "http-request set-header X-Custom 1",
		"### custom annotations end ###",
		"unique-id-header X-Unique-ID",
	}, client.snippets["http"])

	updated = syncFrontendCfgSnippet(t, client, configmap)
	assert.Empty(t, updated, "unchanged snippet should not require a reload")

	updated = syncFrontendCfgSnippet(t, client, map[string]string{"frontend-config-snippet": "unique-id-header X-Request-ID"})
	assert.NotEmpty(t, updated, "changed snippet must require a reload")
	assert.Equal(t, []string{
		"### custom-header ###", "http-request set-header X-Custom 1",
		"### custom annotations end ###",
		"unique-id-header X-Request-ID",
	}, client.snippets["http"])
}
