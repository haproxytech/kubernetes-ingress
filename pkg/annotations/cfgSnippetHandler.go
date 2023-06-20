package annotations

import (
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ConfigSnippetHandler struct{}

func (h ConfigSnippetHandler) Update(k store.K8s, api haproxy.HAProxy, ann Annotations) (reload bool, err error) {
	// We get the configmap configsnippet value
	configmapCfgSnippetValue, errConfigmapCfgSnippet := getConfigmapConfigSnippet(api)
	if errConfigmapCfgSnippet != nil {
		return false, errConfigmapCfgSnippet
	}
	// We pass the configmap config snippet value to be inserted at top of the comment section for every config snippet section
	reload, err = updateConfigSnippet(api, configmapCfgSnippetValue)
	return
}

func getConfigmapConfigSnippet(api api.HAProxyClient) (configmapCfgSnippetValue []string, err error) {
	// configmap config snippet if any.
	configmapCfgSnippetValue = []string{}
	// configmap config snippet will be hold in special backend 'configmap' with origin 'configmap'
	if configmapCfgSnippet := cfgSnippet.backends["configmap"]["configmap"]; configmapCfgSnippet != nil &&
		configmapCfgSnippet.status != store.DELETED &&
		!configmapCfgSnippet.disabled {
		configmapCfgSnippetValue = configmapCfgSnippet.value
		// if any existing and enabled configmap configsnippet then add a special insertion for every existing backend
		// to replicate everywhere the configmap insertion.
		if backends, errGet := api.BackendsGet(); errGet == nil {
			for _, backend := range backends {
				if _, ok := cfgSnippet.backends[backend.Name]; !ok {
					cfgSnippet.backends[backend.Name] = map[string]*cfgData{
						"configmap-insertion": {status: store.ADDED},
					}
				}
			}
		} else {
			err = errGet
		}
	}
	return
}

func updateConfigSnippet(api api.HAProxyClient, configmapCfgSnippetValue []string) (reload bool, err error) {
	errs := utils.Errors{}
	updated := []string{}
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
				if cfgData.status == store.ADDED || cfgData.status == store.MODIFIED {
					logger.Debugf("config snippet from %s has been disabled, reload required", origin)
					reload = true
				}
				continue
			}
			if cfgData.status != store.EMPTY {
				logger.Debugf("config snippet from %s has been created/modified/deleted, reload required", origin)
				reload = true
			}
			// The configsnippet has not been reseen so delete it.
			if cfgData.status == store.DELETED {
				delete(cfgSnippet.backends[backend], origin)
				continue
			}
			if origin != "configmap" && !strings.HasPrefix(origin, SERVICE_NAME_PREFIX) {
				updated = append(updated, cfgData.updated...)
				if len(cfgData.updated) > 0 {
					logger.Debugf("config snippet from %s has been updated, reload required", origin)
				}
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
	reload = reload || len(updated) > 0
	return
}
