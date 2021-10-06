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

package handler

import (
	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	config "github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Refresh struct{}

func (h Refresh) Update(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient) (reload bool, err error) {
	var cleanCrts bool
	cleanCrts, err = annotations.Bool("clean-certs", k.ConfigMaps.Main.Annotations)
	if err != nil {
		cleanCrts = true
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
func (h Refresh) clearBackends(api api.HAProxyClient, cfg *config.ControllerCfg) {
	if cfg.SSLPassthrough {
		// SSL default backend
		cfg.ActiveBackends[cfg.BackSSL] = struct{}{}
	}
	// Ratelimting backends
	for _, rateLimitTable := range cfg.RateLimitTables {
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
