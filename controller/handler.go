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
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type UpdateHandler interface {
	Update(k store.K8s, cfg *Configuration, api api.HAProxyClient) (reload bool, err error)
}

func (c *HAProxyController) initHandlers() {
	c.UpdateHandlers = []UpdateHandler{
		SourceIPHeader{},
		ProxyProtocol{},
		DefaultCertificate{},
		ErrorFile{},
		HTTPS{
			enabled:  !c.osArgs.DisableHTTPS,
			certDir:  HAProxyCertDir,
			ipv4:     !c.osArgs.DisableIPV4,
			addrIpv4: c.osArgs.IPV4BindAddr,
			addrIpv6: c.osArgs.IPV6BindAddr,
			ipv6:     !c.osArgs.DisableIPV6,
			port:     c.osArgs.HTTPSBindPort,
		},
		TCPHandler{},
	}
}
