package annotations

import (
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ConfigSnippetHandler struct{}

func (h ConfigSnippetHandler) Update(k store.K8s, api haproxy.HAProxy, ann Annotations) (err error) {
	// We get the configmap configsnippet value
	configmapCfgSnippetValue := getConfigmapConfigSnippet(k.BackendsWithNoConfigSnippets, api)
	// We pass the configmap config snippet value to be inserted at top of the comment section for every config snippet section
	return updateConfigSnippet(api, configmapCfgSnippetValue)
}

func getConfigmapConfigSnippet(backendsWithNoConfigSnippets map[string]struct{}, api api.HAProxyClient) []string {
	// configmap config snippet if any.
	configmapCfgSnippetValue := []string{}
	// configmap config snippet will be hold in special backend 'configmap' with origin 'configmap'
	if configmapCfgSnippet := cfgSnippet.backends["configmap"]["configmap"]; configmapCfgSnippet != nil &&
		configmapCfgSnippet.status != store.DELETED &&
		!configmapCfgSnippet.disabled {
		configmapCfgSnippetValue = configmapCfgSnippet.value
		// if any existing and enabled configmap configsnippet then add a special insertion for every existing backend
		// to replicate everywhere the configmap insertion.
		for _, backend := range api.BackendsGet() {
			if _, ok := backendsWithNoConfigSnippets[backend.Name]; ok {
				continue
			}
			if _, ok := cfgSnippet.backends[backend.Name]; !ok {
				cfgSnippet.backends[backend.Name] = map[string]*cfgData{
					"configmap-insertion": {status: store.ADDED},
				}
			}
		}
	}
	return configmapCfgSnippetValue
}

func updateConfigSnippet(api api.HAProxyClient, configmapCfgSnippetValue []string) (err error) {
	errs := utils.Errors{}
	// Then we iterate over each backend
	for backend, cfgDataByOrigin := range cfgSnippet.backends {
		// We must remove any previous cfgSnippet insertion.
		if backend != "configmap" {
			err = api.BackendCfgSnippetSet(backend, nil)
		}
		if err != nil {
			errs.Add(err)
			continue
		}
		var serviceCfgSnippetValue []string
		for origin, cfgData := range cfgDataByOrigin {
			//
			if strings.HasPrefix(origin, SERVICE_NAME_PREFIX) && !cfgData.disabled && cfgData.status != store.DELETED {
				serviceCfgSnippetValue = cfgData.value
				break
			}
		}
		// If any configmap config snippet insert it at top level of config snippet.
		cfgSnippetvalue := append([]string{}, configmapCfgSnippetValue...)
		cfgSnippetvalue = append(cfgSnippetvalue, serviceCfgSnippetValue...)
		// Then we can iterate over each config snippet coming from different origin.
		for origin, cfgData := range cfgDataByOrigin {
			if cfgData.disabled {
				instance.ReloadIf(
					cfgData.status == store.ADDED || cfgData.status == store.MODIFIED,
					"config snippet from %s has been disabled", origin)
				continue
			}
			instance.ReloadIf(cfgData.status != store.EMPTY,
				"config snippet from %s has been created/modified/deleted", origin)

			// The configsnippet has not been reseen so delete it.
			if cfgData.status == store.DELETED {
				delete(cfgSnippet.backends[backend], origin)
				continue
			}
			if origin != "configmap" && !strings.HasPrefix(origin, SERVICE_NAME_PREFIX) {
				instance.ReloadIf(len(cfgData.updated) > 0,
					"config snippet from %s has been updated", origin)

				cfgSnippetvalue = append(cfgSnippetvalue, cfgData.value...)
			}
			cfgData.updated = nil
			// Mark the configsnippet to be deleted if not reseen
			// Will be reset to EMPTY state if reseen in Process function.
			cfgData.status = store.DELETED
		}
		if backend != "configmap" {
			// Then insert it.
			err = api.BackendCfgSnippetSet(backend, cfgSnippetvalue)
			if err != nil {
				errs.Add(err)
			}
		}
		// When backend contains no more configsnippet just remove the corresponding map entry
		if len(cfgSnippet.backends[backend]) == 0 {
			delete(cfgSnippet.backends, backend)
		}
	}
	return errs.Result()
}
