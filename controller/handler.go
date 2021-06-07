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
	config "github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/handler"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type UpdateHandler interface {
	Update(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient) (reload bool, err error)
}

func (c *HAProxyController) initHandlers() {
	// handlers executed only once at controller initialization
	logger.Panic(c.clientAPIClosure(c.startupHandlers))

	// handlers executed at reconciliation loop
	c.updateHandlers = []UpdateHandler{
		handler.HTTPS{
			Enabled:  !c.OSArgs.DisableHTTPS,
			CertDir:  c.Cfg.Env.FrontendCertDir,
			IPv4:     !c.OSArgs.DisableIPV4,
			AddrIPv4: c.OSArgs.IPV4BindAddr,
			AddrIPv6: c.OSArgs.IPV6BindAddr,
			IPv6:     !c.OSArgs.DisableIPV6,
			Port:     c.OSArgs.HTTPSBindPort,
		},
		handler.ProxyProtocol{},
		handler.ErrorFile{},
		handler.TCPServices{
			SetDefaultService: c.setDefaultService,
			CertDir:           c.Cfg.Env.FrontendCertDir,
			IPv4:              !c.OSArgs.DisableIPV4,
			AddrIPv4:          c.OSArgs.IPV4BindAddr,
			IPv6:              !c.OSArgs.DisableIPV6,
			AddrIPv6:          c.OSArgs.IPV6BindAddr,
		},
		handler.PatternFiles{},
		handler.Refresh{},
	}
	if c.OSArgs.PprofEnabled {
		c.updateHandlers = append(c.updateHandlers, handler.Pprof{})
	}
	c.updateHandlers = append(c.updateHandlers, handler.Refresh{})
}

func (c *HAProxyController) startupHandlers() error {
	handlers := []UpdateHandler{
		handler.HTTPBind{
			HTTP:      !c.OSArgs.DisableHTTP,
			HTTPS:     !c.OSArgs.DisableHTTPS,
			IPv4:      !c.OSArgs.DisableIPV4,
			IPv6:      !c.OSArgs.DisableIPV6,
			HTTPPort:  c.OSArgs.HTTPBindPort,
			HTTPSPort: c.OSArgs.HTTPSBindPort,
			IPv4Addr:  c.OSArgs.IPV4BindAddr,
			IPv6Addr:  c.OSArgs.IPV6BindAddr,
		}}
	if c.OSArgs.External {
		handlers = append(handlers, handler.GlobalCfg{})
	}
	for _, handler := range handlers {
		_, err := handler.Update(c.Store, &c.Cfg, c.Client)
		if err != nil {
			return err
		}
	}
	return nil
}
