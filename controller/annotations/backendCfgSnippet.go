package annotations

import (
	"strings"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type BackendCfgSnippet struct {
	name    string
	client  api.HAProxyClient
	backend *models.Backend
}

func NewBackendCfgSnippet(n string, c api.HAProxyClient, b *models.Backend) *BackendCfgSnippet {
	return &BackendCfgSnippet{name: n, client: c, backend: b}
}

func (a *BackendCfgSnippet) GetName() string {
	return a.name
}

func (a *BackendCfgSnippet) Process(input string) error {
	var data []string
	for _, line := range strings.Split(strings.Trim(input, "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			data = append(data, line)
		}
	}
	if len(data) == 0 {
		return a.client.BackendCfgSnippetSet(a.backend.Name, nil)
	}
	return a.client.BackendCfgSnippetSet(a.backend.Name, &data)
}
