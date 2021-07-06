package annotations

import (
	"errors"
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

func (a *FrontendCfgSnippet) Parse(input string) error {
	for _, line := range strings.Split(strings.Trim(input, "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			a.data = append(a.data, line)
		}
	}
	if len(a.data) == 0 {
		return errors.New("unable to parse frontend config-snippet: empty input")
	}
	return nil
}

func (a *FrontendCfgSnippet) Update() error {
	switch len(a.data) {
	case 0:
		logger.Infof("Removing config-snippet in %s frontends", strings.Join(a.frontends, ","))
		for _, ft := range a.frontends {
			if err := a.client.FrontendCfgSnippetSet(ft, nil); err != nil {
				return err
			}
		}
	default:
		logger.Infof("Updating config-snippet in %s frontends", strings.Join(a.frontends, ","))
		for _, ft := range a.frontends {
			if err := a.client.FrontendCfgSnippetSet(ft, &a.data); err != nil {
				return err
			}
		}
	}
	return nil
}
