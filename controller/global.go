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
	"github.com/go-test/deep"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/controller/ingress"
	"github.com/haproxytech/kubernetes-ingress/controller/service"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func (c *HAProxyController) handleGlobalConfig() (reload, restart bool) {
	reload, restart = c.globalCfg()
	reload = c.defaultsCfg() || reload
	c.handleDefaultCert()
	reload = c.handleDefaultService() || reload
	(&ingress.Ingress{}).HandleAnnotations(c.Store, &c.Cfg)
	return reload, restart
}

func (c *HAProxyController) globalCfg() (reload, restart bool) {
	var newGlobal, global *models.Global
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
	newGlobal, err = annotations.ModelGlobal("cr-global", c.PodNamespace, c.Store, c.Store.ConfigMaps.Main.Annotations)
	if err != nil {
		logger.Errorf("Global config: %s", err)
	}
	newLg, err = annotations.ModelLog("cr-global", c.PodNamespace, c.Store, c.Store.ConfigMaps.Main.Annotations)
	if err != nil {
		logger.Errorf("Global logging: %s", err)
	}
	if newGlobal == nil {
		newGlobal = &models.Global{}
		for _, a := range annotations.Global(newGlobal, &newLg) {
			err = a.Process(c.Store, c.Store.ConfigMaps.Main.Annotations)
			if err != nil {
				logger.Errorf("annotation %s: %s", a.GetName(), err)
			}
		}
	}
	configuration.SetGlobal(newGlobal, &newLg, c.Cfg.Env)
	updated = deep.Equal(newGlobal, global)
	if len(updated) != 0 {
		logger.Error(c.Client.GlobalPushConfiguration(*newGlobal))
		logger.Debugf("Global config updated: %s\nRestart required", updated)
		restart = true
	}
	updated = deep.Equal(newLg, lg)
	if len(updated) != 0 {
		logger.Error(c.Client.GlobalPushLogTargets(newLg))
		logger.Debugf("Global log targets updated: %s\nRestart required", updated)
		restart = true
	}
	reload, res := c.globalCfgSnipp()
	restart = restart || res
	return
}

func (c *HAProxyController) globalCfgSnipp() (reload, restart bool) {
	var err error
	for _, a := range annotations.GlobalCfgSnipp() {
		err = a.Process(c.Store, c.Store.ConfigMaps.Main.Annotations)
		if err != nil {
			logger.Errorf("annotation %s: %s", a.GetName(), err)
		}
	}
	updatedSnipp, errSnipp := annotations.UpdateGlobalCfgSnippet(c.Client)
	logger.Error(errSnipp)
	if len(updatedSnipp) != 0 {
		logger.Debugf("Global config-snippet updated: %s\nRestart required", updatedSnipp)
		restart = true
	}
	updatedSnipp, errSnipp = annotations.UpdateFrontendCfgSnippet(c.Client, "http", "https", "stats")
	logger.Error(errSnipp)
	if len(updatedSnipp) != 0 {
		logger.Debugf("Frontend config-snippet updated: %s\nReload required", updatedSnipp)
		reload = true
	}
	return
}

func (c *HAProxyController) defaultsCfg() (reload bool) {
	var newDefaults, defaults *models.Defaults
	defaults, err := c.Client.DefaultsGetConfiguration()
	if err != nil {
		logger.Error(err)
		return
	}
	newDefaults, err = annotations.ModelDefaults("cr-defaults", c.PodNamespace, c.Store, c.Store.ConfigMaps.Main.Annotations)
	if err != nil {
		logger.Errorf("Defaults config: %s", err)
	}
	if newDefaults == nil {
		newDefaults = &models.Defaults{}
		for _, a := range annotations.Defaults(newDefaults) {
			logger.Error(a.Process(c.Store, c.Store.ConfigMaps.Main.Annotations))
		}
	}
	configuration.SetDefaults(newDefaults)
	updated := deep.Equal(newDefaults, defaults)
	if len(updated) != 0 {
		if err = c.Client.DefaultsPushConfiguration(*newDefaults); err != nil {
			logger.Error(err)
			return
		}
		reload = true
		logger.Debugf("Defaults config updated: %s\nReload required", updated)
	}
	return
}

// handleDefaultService configures HAProy default backend provided via cli param "default-backend-service"
func (c *HAProxyController) handleDefaultService() (reload bool) {
	var svc *service.Service
	ns, name, err := common.GetK8sPath("default-backend-service", c.Store.ConfigMaps.Main.Annotations)
	if err != nil {
		logger.Errorf("default service: %s", err)
	}
	if name == "" {
		return
	}
	ingressPath := &store.IngressPath{
		SvcNamespace:     ns,
		SvcName:          name,
		IsDefaultBackend: true,
	}
	if svc, err = service.New(c.Store, ingressPath, nil, false, c.Store.ConfigMaps.Main.Annotations); err == nil {
		reload, err = svc.SetDefaultBackend(c.Store, &c.Cfg, c.Client, []string{c.Cfg.FrontHTTP, c.Cfg.FrontHTTPS})
	}
	if err != nil {
		logger.Errorf("default service: %s", err)
	}
	return
}

// handleDefaultCert configures default/fallback HAProxy certificate to use for client HTTPS requests.
func (c *HAProxyController) handleDefaultCert() {
	secret, err := annotations.Secret("ssl-certificate", c.PodNamespace, c.Store, c.Store.ConfigMaps.Main.Annotations)
	if err != nil {
		logger.Errorf("default certificate: %s", err)
		return
	}
	if secret == nil {
		return
	}
	_, err = c.Cfg.Certificates.HandleTLSSecret(secret, certs.FT_DEFAULT_CERT)
	logger.Error(err)
}
