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

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/config"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type GlobalCfg struct {
}

func (handler GlobalCfg) Update(k store.K8s, h haproxy.HAProxy) (reload bool, err error) {
	global := &models.Global{}
	logTargets := &models.LogTargets{}
	config.SetGlobal(global, logTargets, h.Env)
	err = h.GlobalPushConfiguration(*global)
	if err != nil {
		return
	}
	err = h.GlobalPushLogTargets(*logTargets)
	if err != nil {
		return
	}
	defaults := &models.Defaults{}
	config.SetDefaults(defaults)
	err = h.DefaultsPushConfiguration(*defaults)
	if err != nil {
		return
	}
	reload = true
	return
}
