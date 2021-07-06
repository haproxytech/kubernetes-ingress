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
	"reflect"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func (c *HAProxyController) handleGlobalConfig() (reload, restart bool) {
	global, err := c.Client.GlobalGetConfiguration()
	if err != nil {
		logger.Error(err)
		return
	}
	defaults, err := c.Client.DefaultsGetConfiguration()
	if err != nil {
		logger.Error(err)
		return
	}
	oldGlobal := *global
	oldDefaults := *defaults
	annotations.HandleGlobalAnnotations(
		global,
		defaults,
		c.Store,
		c.Client,
		c.Store.ConfigMaps.Main.Annotations,
	)
	if !reflect.DeepEqual(oldGlobal, global) {
		if err = c.Client.GlobalPushConfiguration(global); err != nil {
			logger.Error(err)
			return false, false
		}
		restart = true
	}
	if !reflect.DeepEqual(oldDefaults, defaults) {
		if err = c.Client.DefaultsPushConfiguration(defaults); err != nil {
			logger.Error(err)
			return false, false
		}
		reload = true
	}
	c.handleDefaultCert()
	reload = c.handleDefaultService() || reload

	return reload, restart
}

// handleDefaultService configures HAProy default backend provided via cli param "default-backend-service"
func (c *HAProxyController) handleDefaultService() (reload bool) {
	dsvcData, _ := c.Store.GetValueFromAnnotations("default-backend-service")
	if dsvcData == nil {
		return
	}
	dsvc := strings.Split(dsvcData.Value, "/")

	if len(dsvc) != 2 {
		logger.Errorf("default service '%s': invalid format", dsvcData)
		return
	}
	if dsvc[0] == "" || dsvc[1] == "" {
		return
	}
	namespace, ok := c.Store.Namespaces[dsvc[0]]
	if !ok {
		logger.Errorf("default service '%s': namespace not found" + dsvc[0])
		return
	}
	service, ok := namespace.Services[dsvc[1]]
	if !ok {
		logger.Errorf("default service '%s': service name not found" + dsvc[1])
		return
	}
	ingress := &store.Ingress{
		Namespace:   namespace.Name,
		Name:        "DefaultService",
		Annotations: store.MapStringW{},
		DefaultBackend: &store.IngressPath{
			SvcName:          service.Name,
			SvcPortInt:       service.Ports[0].Port,
			IsDefaultBackend: true,
		},
	}
	reload, err := c.setDefaultService(ingress, []string{c.Cfg.FrontHTTP, c.Cfg.FrontHTTPS})
	if err != nil {
		logger.Errorf("default service '%s/%s': %s", namespace.Name, service.Name, err)
		return
	}
	return reload
}

// handleDefaultCert configures default/fallback HAProxy certificate to use for client HTTPS requests.
func (c *HAProxyController) handleDefaultCert() {
	secretAnn, _ := c.Store.GetValueFromAnnotations("ssl-certificate", c.Store.ConfigMaps.Main.Annotations)
	if secretAnn == nil {
		return
	}
	_, err := c.Cfg.Certificates.HandleTLSSecret(c.Store, haproxy.SecretCtx{
		SecretPath: secretAnn.Value,
		SecretType: haproxy.FT_DEFAULT_CERT,
	})
	logger.Error(err)
}
