package service

import (
	"strings"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type CfgSnippet struct {
	name    string
	client  api.HAProxyClient
	backend *models.Backend
}

func NewCfgSnippet(n string, c api.HAProxyClient, b *models.Backend) *CfgSnippet {
	return &CfgSnippet{name: n, client: c, backend: b}
}

func (a *CfgSnippet) GetName() string {
	return a.name
}

func (a *CfgSnippet) Process(input string) error {
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
