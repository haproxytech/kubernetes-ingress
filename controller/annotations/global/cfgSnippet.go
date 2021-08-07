package global

import (
	"strings"

	"github.com/haproxytech/config-parser/v4/types"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type CfgSnippet struct {
	name   string
	data   []string
	client api.HAProxyClient
}

func NewCfgSnippet(n string, c api.HAProxyClient) *CfgSnippet {
	return &CfgSnippet{name: n, client: c}
}

func (a *CfgSnippet) GetName() string {
	return a.name
}

func (a *CfgSnippet) Process(input string) error {
	for _, line := range strings.Split(strings.Trim(input, "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			a.data = append(a.data, line)
		}
	}
	if len(a.data) == 0 {
		return a.client.GlobalCfgSnippet(nil)
	}
	return a.client.GlobalCfgSnippet(&types.StringSliceC{Value: a.data})
}
