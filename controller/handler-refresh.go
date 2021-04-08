// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type RefreshHandler struct{}

func (h RefreshHandler) Update(k store.K8s, cfg *Configuration, api api.HAProxyClient) (reload bool, err error) {
	cleanCrts := true
	if cleanCrtAnn, _ := k.GetValueFromAnnotations("clean-certs", k.ConfigMaps.Main.Annotations); cleanCrtAnn != nil && cleanCrtAnn.Status != DELETED {
		cleanCrts, err = utils.GetBoolValue(cleanCrtAnn.Value, "clean-certs")
	}
	if cleanCrts {
		reload = cfg.Certificates.Refresh() || reload
	}
	reload = cfg.HAProxyRules.Refresh(api) || reload
	reload = cfg.MapFiles.Refresh(api) || reload
	h.clearBackends(api, cfg)
	return
}

// Remove unused backends
func (h RefreshHandler) clearBackends(api api.HAProxyClient, cfg *Configuration) {
	if cfg.SSLPassthrough {
		// SSL default backend
		cfg.ActiveBackends[SSLDefaultBaceknd] = struct{}{}
	}
	// Ratelimting backends
	for _, rateLimitTable := range rateLimitTables {
		cfg.ActiveBackends[rateLimitTable] = struct{}{}
	}
	allBackends, err := api.BackendsGet()
	if err != nil {
		return
	}
	for _, backend := range allBackends {
		if _, ok := cfg.ActiveBackends[backend.Name]; !ok {
			logger.Debugf("Deleting backend '%s'", backend.Name)
			if err := api.BackendDelete(backend.Name); err != nil {
				logger.Panic(err)
			}
		}
	}
}
