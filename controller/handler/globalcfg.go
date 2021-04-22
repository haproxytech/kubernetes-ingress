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
	"github.com/haproxytech/config-parser/v3/params"
	"github.com/haproxytech/config-parser/v3/types"

	config "github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type GlobalCfg struct {
}

func (h GlobalCfg) Update(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient) (reload bool, err error) {
	var errors utils.Errors
	errors.Add(
		// Configure runtime socket
		api.RuntimeSocket(nil),
		api.RuntimeSocket(&types.Socket{
			Path: cfg.Env.RuntimeSocket,
			Params: []params.BindOption{
				&params.BindOptionDoubleWord{Name: "expose-fd", Value: "listeners"},
				&params.BindOptionValue{Name: "level", Value: "admin"},
			},
		}),
		// Configure pidfile
		api.PIDFile(&types.StringC{Value: cfg.Env.PIDFile}),
		// Configure server-state-base
		api.ServerStateBase(&types.StringC{Value: cfg.Env.StateDir}),
	)
	err = errors.Result()
	reload = true
	return
}
