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
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type Refresh struct{}

func (handler Refresh) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (reload bool, err error) {
	var cleanCrts = true
	cleanCrtsAnn, _ := annotations.ParseBool("clean-certs", k.ConfigMaps.Main.Annotations)
	// cleanCrtsAnn is empty if clean-certs not set or set with a non boolean value =>  error
	if cleanCrtsAnn == "false" {
		cleanCrts = false
	}
	// Certs
	if cleanCrts {
		reload = h.RefreshCerts()
	}
	// Rules
	reload = h.RefreshRules(h.HAProxyClient) || reload
	// Maps
	reload = h.RefreshMaps(h.HAProxyClient) || reload
	// Backends
	deleted, err := h.RefreshBackends()
	logger.Error(err)
	for _, backend := range deleted {
		logger.Debugf("Backend '%s' deleted", backend)
		annotations.RemoveBackendCfgSnippet(backend)
	}
	return
}
