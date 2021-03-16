package annotations

import (
	"errors"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type FrontendCfgSnippet struct {
	name      string
	data      []string
	client    api.HAProxyClient
	frontends []string
}

func NewFrontendCfgSnippet(n string, c api.HAProxyClient, frontendNames []string) *FrontendCfgSnippet {
	return &FrontendCfgSnippet{name: n, client: c, frontends: frontendNames}
}

func (a *FrontendCfgSnippet) GetName() string {
	return a.name
}

func (a *FrontendCfgSnippet) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	if input.Status == store.DELETED {
		return nil
	}
	for _, line := range strings.Split(strings.Trim(input.Value, "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			a.data = append(a.data, line)
		}
	}
	if len(a.data) == 0 {
		return errors.New("unable to parse config-snippet: empty input")
	}
	return nil
}

func (a *FrontendCfgSnippet) Update() error {
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
