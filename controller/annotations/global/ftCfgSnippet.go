package global

import (
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type FrontendCfgSnippet struct {
	name      string
	data      []string
	frontends []string
	client    api.HAProxyClient
}

func NewFrontendCfgSnippet(n string, c api.HAProxyClient, frontendNames []string) *FrontendCfgSnippet {
	return &FrontendCfgSnippet{name: n, client: c, frontends: frontendNames}
}

func (a *FrontendCfgSnippet) GetName() string {
	return a.name
}

func (a *FrontendCfgSnippet) Process(input string) error {
	for _, line := range strings.Split(strings.Trim(input, "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			a.data = append(a.data, line)
		}
	}
	switch len(a.data) {
	case 0:
		for _, ft := range a.frontends {
			if err := a.client.FrontendCfgSnippetSet(ft, nil); err != nil {
				return err
			}
		}
	default:
		for _, ft := range a.frontends {
			if err := a.client.FrontendCfgSnippetSet(ft, &a.data); err != nil {
				return err
			}
		}
	}
	return nil
}
