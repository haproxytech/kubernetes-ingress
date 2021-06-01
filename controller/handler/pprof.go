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
	"github.com/haproxytech/client-native/v2/models"

	config "github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/route"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Pprof struct {
}

func (h Pprof) Update(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient) (reload bool, err error) {
	pprofBackend := "pprof"

	_, err = api.BackendGet(pprofBackend)
	if err != nil {
		err = api.BackendCreate(models.Backend{
			Name: pprofBackend,
			Mode: "http",
		})
		if err != nil {
			return
		}
		err = api.BackendServerCreate(pprofBackend, models.Server{
			Name:    "pprof",
			Address: "127.0.0.1:6060",
		})
		if err != nil {
			return
		}
		logger.Debug("pprof backend created")
	}
	err = route.AddHostPathRoute(route.Route{
		BackendName: pprofBackend,
		Path: &store.IngressPath{
			Path:           "/debug/pprof",
			ExactPathMatch: false,
		},
	}, cfg.MapFiles)
	if err != nil {
		return
	}
	cfg.ActiveBackends[pprofBackend] = struct{}{}
	reload = true
	return
}
