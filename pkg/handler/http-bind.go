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
	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type HTTPBind struct {
	IPv4Addr  string
	IPv6Addr  string
	HTTPPort  int64
	HTTPSPort int64
	HTTP      bool
	HTTPS     bool
	IPv4      bool
	IPv6      bool
}

func (handler HTTPBind) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	var errors utils.Errors
	frontends := make(map[string]int64, 2)
	protos := make(map[string]string, 2)
	if handler.HTTP {
		frontends[h.FrontHTTP] = handler.HTTPPort
	}
	if handler.HTTPS {
		frontends[h.FrontHTTPS] = handler.HTTPSPort
	}
	if handler.IPv4 {
		protos["v4"] = handler.IPv4Addr
	}
	if handler.IPv6 {
		protos["v6"] = handler.IPv6Addr

		// IPv6 not disabled, so add v6 listening to stats frontend
		errors.Add(h.FrontendBindCreate("stats",
			models.Bind{
				BindParams: models.BindParams{
					Name: "v6",
					V4v6: false,
				},
				Address: ":::1024",
			}))
	}
	for ftName, ftPort := range frontends {
		for proto, addr := range protos {
			bind := models.Bind{
				BindParams: models.BindParams{
					Name: proto,
				},
				Address: addr,
				Port:    utils.PtrInt64(ftPort),
			}
			if err = h.FrontendBindEdit(ftName, bind); err != nil {
				errors.Add(h.FrontendBindCreate(ftName, bind))
			}
		}
	}
	err = errors.Result()
	instance.Reload("New HTTP(S) bindings")
	return err
}
