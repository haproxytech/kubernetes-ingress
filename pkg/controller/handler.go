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
	"github.com/haproxytech/kubernetes-ingress/pkg/handler"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type UpdateHandler interface {
	Update(k store.K8s, h haproxy.HAProxy) (reload bool, err error)
}

func (c *HAProxyController) initHandlers() {
	// handlers executed only once at controller initialization
	logger.Panic(c.clientAPIClosure(c.startupHandlers))

	// handlers executed at reconciliation loop
	c.updateHandlers = []UpdateHandler{
		handler.HTTPS{
			Enabled:  !c.osArgs.DisableHTTPS,
			CertDir:  c.haproxy.Certs.FrontendDir,
			IPv4:     !c.osArgs.DisableIPV4,
			AddrIPv4: c.osArgs.IPV4BindAddr,
			AddrIPv6: c.osArgs.IPV6BindAddr,
			IPv6:     !c.osArgs.DisableIPV6,
			Port:     c.osArgs.HTTPSBindPort,
		},
		handler.ProxyProtocol{},
		&handler.ErrorFiles{},
		handler.TCPServices{
			CertDir:  c.haproxy.Certs.FrontendDir,
			IPv4:     !c.osArgs.DisableIPV4,
			AddrIPv4: c.osArgs.IPV4BindAddr,
			IPv6:     !c.osArgs.DisableIPV6,
			AddrIPv6: c.osArgs.IPV6BindAddr,
		},
		&handler.PatternFiles{},
		handler.Refresh{},
	}
	if c.osArgs.PprofEnabled {
		c.updateHandlers = append(c.updateHandlers, handler.Pprof{})
	}
	c.updateHandlers = append(c.updateHandlers, handler.Refresh{})
}

func (c *HAProxyController) startupHandlers() error {
	handlers := []UpdateHandler{
		handler.GlobalCfg{},
		handler.HTTPBind{
			HTTP:      !c.osArgs.DisableHTTP,
			HTTPS:     !c.osArgs.DisableHTTPS,
			IPv4:      !c.osArgs.DisableIPV4,
			IPv6:      !c.osArgs.DisableIPV6,
			HTTPPort:  c.osArgs.HTTPBindPort,
			HTTPSPort: c.osArgs.HTTPSBindPort,
			IPv4Addr:  c.osArgs.IPV4BindAddr,
			IPv6Addr:  c.osArgs.IPV6BindAddr,
		}}
	for _, handler := range handlers {
		_, err := handler.Update(c.store, c.haproxy)
		if err != nil {
			return err
		}
	}
	return nil
}
