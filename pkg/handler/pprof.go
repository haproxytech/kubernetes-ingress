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
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/route"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

const pprofBackend = "pprof"

type Pprof struct{}

func (handler Pprof) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	k.BackendsWithNoConfigSnippets[pprofBackend] = struct{}{}

	if !h.BackendExists(pprofBackend) {
		h.BackendCreatePermanently(models.Backend{
			BackendBase: models.BackendBase{
				Name: pprofBackend,
				Mode: "http",
			},
		})
		err = h.BackendServerCreateOrUpdate(pprofBackend, models.Server{
			Name:    pprofBackend,
			Address: fmt.Sprintf("127.0.0.1:%d", h.Env.ControllerPort),
		})
		if err != nil {
			return
		}
		logger.Debug("pprof backend created")
	}

	err = route.AddHostPathRoute(route.Route{
		BackendName: pprofBackend,
		Path: &store.IngressPath{
			Path:          "/debug/pprof",
			PathTypeMatch: store.PATH_TYPE_IMPLEMENTATION_SPECIFIC,
		},
	}, h.Maps)
	if err != nil {
		return
	}

	return
}
