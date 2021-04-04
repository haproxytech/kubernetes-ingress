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

package controller

import (
	"net"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ProxyProtocol struct{}

func (p ProxyProtocol) Update(k store.K8s, cfg *Configuration, api api.HAProxyClient) (reload bool, err error) {
	//  Get annotation status
	annProxyProtocol, _ := k.GetValueFromAnnotations("proxy-protocol", k.ConfigMaps.Main.Annotations)
	if annProxyProtocol == nil {
		return false, nil
	}
	if annProxyProtocol.Status == DELETED {
		logger.Trace("Deleting ProxyProtocol configuration")
		return false, nil
	}
	// Validate annotation
	mapName := "proxy-protocol-" + utils.Hash([]byte(annProxyProtocol.Value))
	if !cfg.MapFiles.Exists(mapName) {
		for _, address := range strings.Split(annProxyProtocol.Value, ",") {
			address = strings.TrimSpace(address)
			if ip := net.ParseIP(address); ip == nil {
				if _, _, err = net.ParseCIDR(address); err != nil {
					logger.Errorf("incorrect address '%s' in proxy-protocol annotation", address)
					continue
				}
				cfg.MapFiles.AppendRow(mapName, address)
			}
		}
	}
	// Configure Annotation
	logger.Trace("Configuring ProxyProtcol annotation")
	frontends := []string{FrontendHTTP, FrontendHTTPS}
	if cfg.SSLPassthrough {
		frontends = []string{FrontendHTTP, FrontendSSL}
	}
	err = cfg.HAProxyRules.AddRule(rules.ReqProxyProtocol{SrcIPsMap: mapName}, "", frontends...)
	return false, err
}
