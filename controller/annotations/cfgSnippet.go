package annotations

import (
	"reflect"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type CfgSnippet struct {
	name     string
	frontend string
	backend  string
}

type cfgData struct {
	value    []string
	toUpdate bool
}

var cfgSnippet struct {
	global    *cfgData
	frontends map[string]*cfgData
	backends  map[string]*cfgData
}

//nolint: gochecknoinits
func init() {
	cfgSnippet.global = &cfgData{}
	cfgSnippet.frontends = make(map[string]*cfgData)
	cfgSnippet.backends = make(map[string]*cfgData)
}

func NewGlobalCfgSnippet(n string) *CfgSnippet {
	return &CfgSnippet{name: n}
}

func NewFrontendCfgSnippet(n string, f string) *CfgSnippet {
	return &CfgSnippet{name: n, frontend: f}
}

func NewBackendCfgSnippet(n string, b string) *CfgSnippet {
	return &CfgSnippet{name: n, backend: b}
}

func (a *CfgSnippet) GetName() string {
	return a.name
}

func (a *CfgSnippet) Process(input string) error {
	data := strings.Split(strings.Trim(input, "\n"), "\n")
	switch {
	case a.frontend != "":
		_, ok := cfgSnippet.frontends[a.frontend]
		if !ok {
			cfgSnippet.frontends[a.frontend] = &cfgData{}
		}
		if !reflect.DeepEqual(cfgSnippet.frontends[a.frontend].value, data) {
			cfgSnippet.frontends[a.frontend].value = data
			cfgSnippet.frontends[a.frontend].toUpdate = true
		}
	case a.backend != "":
		_, ok := cfgSnippet.backends[a.backend]
		if !ok {
			cfgSnippet.backends[a.backend] = &cfgData{}
		}
		if !reflect.DeepEqual(cfgSnippet.backends[a.backend].value, data) {
			cfgSnippet.backends[a.backend].value = data
			cfgSnippet.backends[a.backend].toUpdate = true
		}
	default:
		if !reflect.DeepEqual(cfgSnippet.global.value, data) {
			cfgSnippet.global.value = data
			cfgSnippet.global.toUpdate = true
		}
	}
	return nil
}

func UpdateGlobalCfgSnippet(api api.HAProxyClient) (updated bool, err error) {
	if !cfgSnippet.global.toUpdate {
		return
	}
	err = api.GlobalCfgSnippet(cfgSnippet.global.value)
	if err != nil {
		return
	}
	updated = true
	cfgSnippet.global.toUpdate = false
	return
}

func UpdateFrontendCfgSnippet(api api.HAProxyClient, frontends ...string) (updated bool, err error) {
	for _, ft := range frontends {
		data, ok := cfgSnippet.frontends[ft]
		if !ok {
			continue
		}
		if !data.toUpdate {
			continue
		}
		err = api.FrontendCfgSnippetSet(ft, data.value)
		if err != nil {
			return
		}
		updated = true
		data.toUpdate = false
	}
	return
}

func UpdateBackendCfgSnippet(api api.HAProxyClient, backends ...string) (updated bool, err error) {
	for _, bnd := range backends {
		data, ok := cfgSnippet.backends[bnd]
		if !ok {
			continue
		}
		if !data.toUpdate {
			continue
		}
		err = api.BackendCfgSnippetSet(bnd, data.value)
		if err != nil {
			return
		}
		updated = true
		data.toUpdate = false
		cfgSnippet.backends[bnd] = data
	}
	return
}
