package annotations

import (
	"strings"

	"github.com/go-test/deep"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type CfgSnippet struct {
	name     string
	frontend string
	backend  string
}

type cfgData struct {
	value         []string
	backendValue  map[string]struct{}
	previousValue []string
	updated       []string
}

// cfgSnippet is a particular type of config that is not
// handled by the upstram library haproxytech/client-native.
// Which means there is no client-native models to
// store, exchange and query cfgSnippet Data. Thus this logic
// is directly handled by Ingress Controller in this package.
//
// The code in this file need to be rewritten to avoid init,
// global variables and rather expose a clean interface.
var cfgSnippet struct {
	global    *cfgData
	frontends map[string]*cfgData
	backends  map[string]*cfgData
}

//nolint:gochecknoinits
func init() {
	cfgSnippet.global = &cfgData{backendValue: map[string]struct{}{}}
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

func (a *CfgSnippet) Process(k store.K8s, annotations ...map[string]string) error {
	var data []string
	input := common.GetValue(a.GetName(), annotations...)
	if input != "" {
		data = strings.Split(strings.Trim(input, "\n"), "\n")
	}
	switch {
	case a.frontend != "":
		_, ok := cfgSnippet.frontends[a.frontend]
		if !ok {
			cfgSnippet.frontends[a.frontend] = &cfgData{}
		}
		updated := deep.Equal(cfgSnippet.frontends[a.frontend].value, data)
		if len(updated) != 0 {
			cfgSnippet.frontends[a.frontend].value = data
			cfgSnippet.frontends[a.frontend].updated = updated
		}
	case a.backend != "":
		cfg, ok := cfgSnippet.backends[a.backend]
		if !ok {
			cfg = &cfgData{}
		}
		if cfg.backendValue == nil {
			cfg.backendValue = map[string]struct{}{}
		}
		// As a configsnippet from a service referenced by multiple ingress will be resubmited as many times as ingresses
		// We guarantee unicity with map.
		cfg.backendValue[input] = struct{}{}
		cfgSnippet.backends[a.backend] = cfg
	default:
		updated := deep.Equal(cfgSnippet.global.value, data)
		if len(updated) != 0 {
			cfgSnippet.global.value = data
			cfgSnippet.global.updated = updated
		}
	}
	return nil
}

func UpdateGlobalCfgSnippet(api api.HAProxyClient) (updated []string, err error) {
	if len(cfgSnippet.global.updated) == 0 {
		return
	}
	err = api.GlobalCfgSnippet(cfgSnippet.global.value)
	if err != nil {
		return
	}
	updated = cfgSnippet.global.updated
	cfgSnippet.global.updated = nil
	return
}

func UpdateFrontendCfgSnippet(api api.HAProxyClient, frontends ...string) (updated []string, err error) {
	for _, ft := range frontends {
		data, ok := cfgSnippet.frontends[ft]
		if !ok {
			continue
		}
		if len(data.updated) == 0 {
			continue
		}
		err = api.FrontendCfgSnippetSet(ft, data.value)
		if err != nil {
			return
		}
		updated = data.updated
		data.updated = nil
		cfgSnippet.frontends[ft] = data
	}
	return
}

func UpdateBackendCfgSnippet(k store.K8s, api api.HAProxyClient, backend string) (updated []string, err error) {
	data, ok := cfgSnippet.backends[backend]
	if !ok {
		return
	}

	defer func() {
		data.value = nil
		data.backendValue = nil
	}()

	var value []string
	for input := range data.backendValue {
		if input != "" {
			data := strings.Split(strings.Trim(input, "\n"), "\n")
			value = append(value, data...)
		}
	}
	updated = deep.Equal(data.previousValue, value)
	if len(updated) == 0 {
		return
	}
	err = api.BackendCfgSnippetSet(backend, value)
	if err != nil {
		return
	}
	data.previousValue = value
	return
}

func RemoveBackendCfgSnippet(backend string) {
	if cfgSnippet.backends == nil {
		return
	}
	delete(cfgSnippet.backends, backend)
}

func HandleBackendCfgSnippet(k store.K8s, api api.HAProxyClient) (reload bool, err error) {
	var errs utils.Errors
	for backend := range cfgSnippet.backends {
		updated, errBackend := UpdateBackendCfgSnippet(k, api, backend)
		if len(updated) != 0 {
			logger.Debugf("backend configsnippet of '%s' has been updated, reload required", backend)
		}
		reload = reload || len(updated) != 0
		errs.Add(errBackend)
	}
	err = errs.Result()
	return
}
