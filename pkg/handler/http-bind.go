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

	config "github.com/haproxytech/kubernetes-ingress/pkg/configuration"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type HTTPBind struct {
	HTTP      bool
	HTTPS     bool
	IPv4      bool
	IPv6      bool
	HTTPPort  int64
	HTTPSPort int64
	IPv4Addr  string
	IPv6Addr  string
}

func (h HTTPBind) Update(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient) (reload bool, err error) {
	var errors utils.Errors
	frontends := make(map[string]int64, 2)
	protos := make(map[string]string, 2)
	if h.HTTP {
		frontends[cfg.FrontHTTP] = h.HTTPPort
	}
	if h.HTTPS {
		frontends[cfg.FrontHTTPS] = h.HTTPSPort
	}
	if h.IPv4 {
		protos["v4"] = h.IPv4Addr
	}
	if h.IPv6 {
		protos["v6"] = h.IPv6Addr

		// IPv6 not disabled, so add v6 listening to stats frontend
		errors.Add(api.FrontendBindCreate("stats",
			models.Bind{
				Name:    "v6",
				Address: ":::1024",
				V4v6:    false,
			}))
	}
	for ftName, ftPort := range frontends {
		for proto, addr := range protos {
			bind := models.Bind{
				Name:    proto,
				Address: addr,
				Port:    utils.PtrInt64(ftPort),
			}
			if err = api.FrontendBindEdit(ftName, bind); err != nil {
				errors.Add(api.FrontendBindCreate(ftName, bind))
			}
		}
	}
	err = errors.Result()
	reload = true
	return
}
