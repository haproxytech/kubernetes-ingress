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

	"github.com/go-test/deep"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func (c *HAProxyController) handleGlobalConfig() (reload, restart bool) {
	restart = c.globalCfg()
	reload = c.defaultsCfg()
	c.handleDefaultCert()
	reload = c.handleDefaultService() || reload
	_ = c.handleIngressAnnotations(store.Ingress{})
	return reload, restart
}

func (c *HAProxyController) globalCfg() (restart bool) {
	var global *models.Global
	var newGlobal models.Global
	var newLg models.LogTargets
	var err error
	var updated []string
	global, err = c.Client.GlobalGetConfiguration()
	if err != nil {
		logger.Error(err)
		return
	}
	lg, errL := c.Client.GlobalGetLogTargets()
	if errL != nil {
		logger.Error(errL)
		return
	}
	newGlobal = *global
	for _, a := range annotations.GetGlobalAnnotations(&newGlobal, &newLg) {
		annValue := annotations.GetValue(a.GetName(), c.Store.ConfigMaps.Main.Annotations)
		err = a.Process(annValue)
		if err != nil {
			logger.Errorf("annotation %s: %s", a.GetName(), err)
		}
	}
	updated = deep.Equal(newGlobal, *global)
	if len(updated) != 0 {
		logger.Error(c.Client.GlobalPushConfiguration(newGlobal))
		logger.Debugf("Global config updated: %s\nRestart required", updated)
		restart = true
	}
	updated = deep.Equal(newLg, lg)
	if len(updated) != 0 {
		c.Client.GlobalDeleteLogTargets()
		logger.Error(c.Client.GlobalCreateLogTargets(newLg))
		logger.Debugf("Syslog servers updated: %s\nRestart required", updated)
		restart = true
	}
	change, errSnipp := annotations.UpdateGlobalCfgSnippet(c.Client)
	logger.Error(errSnipp)
	restart = restart || change
	change, errSnipp = annotations.UpdateFrontendCfgSnippet(c.Client, "http", "https", "stats")
	logger.Error(errSnipp)
	restart = restart || change
	return
}

func (c *HAProxyController) defaultsCfg() (reload bool) {
	var oldDefaults models.Defaults
	defaults, err := c.Client.DefaultsGetConfiguration()
	if err != nil {
		logger.Error(err)
		return
	}
	oldDefaults = *defaults
	for _, a := range annotations.GetDefaultsAnnotations(defaults) {
		annValue := annotations.GetValue(a.GetName(), c.Store.ConfigMaps.Main.Annotations)
		logger.Error(a.Process(annValue))
	}
	result := deep.Equal(&oldDefaults, defaults)
	if len(result) != 0 {
		if err = c.Client.DefaultsPushConfiguration(defaults); err != nil {
			logger.Error(err)
			return
		}
		reload = true
		logger.Debugf("Defaults config updated: %s\nReload required", result)
	}
	return
}

// handleDefaultService configures HAProy default backend provided via cli param "default-backend-service"
func (c *HAProxyController) handleDefaultService() (reload bool) {
	dsvcData := annotations.GetValue("default-backend-service")
	if dsvcData == "" {
		return
	}
	dsvc := strings.Split(dsvcData, "/")

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
		Annotations: map[string]string{},
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
	secretAnn := annotations.GetValue("ssl-certificate", c.Store.ConfigMaps.Main.Annotations)
	if secretAnn == "" {
		return
	}
	_, err := c.Cfg.Certificates.HandleTLSSecret(c.Store, haproxy.SecretCtx{
		SecretPath: secretAnn,
		SecretType: haproxy.FT_DEFAULT_CERT,
	})
	logger.Error(err)
}
