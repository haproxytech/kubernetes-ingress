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
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type GlobalCfg struct {
}

func (h GlobalCfg) Update(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient) (reload bool, err error) {
	global, err := api.GlobalGetConfiguration()
	if err != nil {
		return
	}
	global.Pidfile = cfg.Env.PIDFile
	global.ServerStateBase = cfg.Env.StateDir
	global.RuntimeAPIs = []*models.RuntimeAPI{
		{
			Address:           &cfg.Env.RuntimeSocket,
			ExposeFdListeners: true,
			Level:             "admin",
		},
	}
	err = api.GlobalPushConfiguration(global)
	if err != nil {
		return
	}
	reload = true
	return
}
