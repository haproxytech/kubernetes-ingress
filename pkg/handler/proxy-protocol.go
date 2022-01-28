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
	"net"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ProxyProtocol struct{}

func (handler ProxyProtocol) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (reload bool, err error) {
	//  Get annotation status
	annProxyProtocol := a.String("proxy-protocol", k.ConfigMaps.Main.Annotations)
	if annProxyProtocol == "" {
		return false, nil
	}
	// Validate annotation
	mapName := maps.Name("proxy-protocol-" + utils.Hash([]byte(annProxyProtocol)))
	if !h.MapFiles.Exists(mapName) {
		for _, address := range strings.Split(annProxyProtocol, ",") {
			address = strings.TrimSpace(address)
			if ip := net.ParseIP(address); ip == nil {
				if _, _, err = net.ParseCIDR(address); err != nil {
					logger.Errorf("incorrect address '%s' in proxy-protocol annotation", address)
					continue
				}
			}
			h.MapFiles.AppendRow(mapName, address)
		}
	}
	// Configure Annotation
	logger.Trace("Configuring ProxyProtcol annotation")
	frontends := []string{h.FrontHTTP, h.FrontHTTPS}
	if h.SSLPassthrough {
		frontends = []string{h.FrontHTTP, h.FrontSSL}
	}
	for _, frontend := range frontends {
		err = h.AddRule(rules.ReqProxyProtocol{SrcIPsMap: maps.GetPath(mapName)}, false, frontend)
		if err != nil {
			return
		}
	}
	return
}
