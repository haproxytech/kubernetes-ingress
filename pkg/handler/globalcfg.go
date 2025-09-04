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
	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/controller/constants"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type GlobalCfg struct{}

func (handler GlobalCfg) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	global := &models.Global{}
	logTargets := &models.LogTargets{}
	env.SetGlobal(global, logTargets, h.Env)
	err = h.GlobalPushConfiguration(*global)
	if err != nil {
		return err
	}
	err = h.GlobalPushLogTargets(*logTargets)
	if err != nil {
		return err
	}
	defaults := &models.Defaults{}
	env.SetDefaults(defaults)
	defaults.Name = constants.DefaultsSectionName
	err = h.DefaultsPushConfiguration(*defaults)
	if err != nil {
		return err
	}
	instance.Reload("new global configuration applied")
	return err
}
