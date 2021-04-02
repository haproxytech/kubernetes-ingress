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
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func (c *HAProxyController) handleGlobalConfig() (reload, restart bool) {
	restart, reload = annotations.HandleGlobalAnnotations(
		c.Store,
		c.Client,
		false,
		c.Store.ConfigMaps.Main.Annotations,
	)
	reload = c.handleDefaultCert() || reload
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
		logger.Errorf("default service '%s': invalid format", dsvcData.Value)
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
	reload, err := c.setDefaultService(ingress, []string{FrontendHTTP, FrontendHTTPS})
	if err != nil {
		logger.Errorf("default service '%s/%s': %s", namespace.Name, service.Name, err)
		return
	}
	return reload
}

// handleDefaultCert configures default/fallback HAProxy certificate to use for client HTTPS requests.
func (c *HAProxyController) handleDefaultCert() (reload bool) {
	secretAnn, _ := c.Store.GetValueFromAnnotations("ssl-certificate", c.Store.ConfigMaps.Main.Annotations)
	if secretAnn == nil {
		return false
	}
	crt, updated, _ := c.cfg.Certificates.HandleTLSSecret(c.Store, haproxy.SecretCtx{
		SecretPath: secretAnn.Value,
		SecretType: haproxy.FT_DEFAULT_CERT,
	})
	return crt != "" && updated
}
