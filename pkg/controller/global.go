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
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/controller/constants"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/service"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func (c *HAProxyController) handleGlobalConfig() {
	c.globalCfg()
	c.defaultsCfg()
	c.handleDefaultCert()
	if c.osArgs.DefaultBackendService.String() == "" {
		c.handleDefaultLocalService()
	} else {
		c.handleDefaultService()
	}
	ingress.HandleCfgMapAnnotations(c.store, c.haproxy, c.annotations)
}

func (c *HAProxyController) globalCfg() {
	var newGlobal, global *models.Global
	var newLg models.LogTargets
	var err error
	global, err = c.haproxy.GlobalGetConfiguration()
	if err != nil {
		logger.Error(err)
		return
	}
	lg, errL := c.haproxy.GlobalGetLogTargets()
	if errL != nil {
		logger.Error(errL)
		return
	}
	newGlobal, err = annotations.ModelGlobal("cr-global", c.podNamespace, c.store, c.store.ConfigMaps.Main.Annotations)
	if err != nil {
		logger.Errorf("Global config: %s", err)
	}
	newLg, err = annotations.ModelLog("cr-global", c.podNamespace, c.store, c.store.ConfigMaps.Main.Annotations)
	if err != nil {
		logger.Errorf("Global logging: %s", err)
	}
	if newGlobal == nil {
		newGlobal = &models.Global{
			// TuneSslDefaultDhParam: 2048,
			TuneOptions: &models.GlobalTuneOptions{
				SslDefaultDhParam: 2048,
			},
		}
		for _, a := range c.annotations.Global(newGlobal, &newLg) {
			err = a.Process(c.store, c.store.ConfigMaps.Main.Annotations)
			if err != nil {
				logger.Errorf("annotation %s: %s", a.GetName(), err)
			}
		}
	}
	if newGlobal.TuneOptions == nil {
		newGlobal.TuneOptions = &models.GlobalTuneOptions{}

		if newGlobal.TuneSslDefaultDhParam == 0 {
			newGlobal.TuneOptions.SslDefaultDhParam = 2048
		}
	}
	env.SetGlobal(newGlobal, &newLg, c.haproxy.Env)
	diff := newGlobal.Diff(*global)
	if len(diff) != 0 {
		logger.Error(c.haproxy.GlobalPushConfiguration(*newGlobal))
		instance.Restart("Global config updated: %+v", diff)
	}
	diff = newLg.Diff(lg)
	if len(diff) != 0 {
		logger.Error(c.haproxy.GlobalPushLogTargets(newLg))
		instance.Restart("Global log targets updated: %+v", diff)
	}
	c.globalCfgSnipp()
}

func (c *HAProxyController) globalCfgSnipp() {
	var err error
	for _, a := range c.annotations.GlobalCfgSnipp() {
		err = a.Process(c.store, c.store.ConfigMaps.Main.Annotations)
		if err != nil {
			logger.Errorf("annotation %s: %s", a.GetName(), err)
		}
	}
	updatedSnipp, errSnipp := annotations.UpdateGlobalCfgSnippet(c.haproxy)
	logger.Error(errSnipp)
	instance.RestartIf(len(updatedSnipp) != 0,
		"Global config-snippet updated: %s", strings.Join(updatedSnipp, "\n"))

	updatedSnipp, errSnipp = annotations.UpdateFrontendCfgSnippet(c.haproxy, "http", "https", "stats")
	logger.Error(errSnipp)
	instance.ReloadIf(len(updatedSnipp) != 0,
		"Frontend config-snippet updated: %s", strings.Join(updatedSnipp, "\n"))
}

func (c *HAProxyController) defaultsCfg() {
	var newDefaults, defaults *models.Defaults
	defaults, err := c.haproxy.DefaultsGetConfiguration()
	if err != nil {
		logger.Error(err)
		return
	}
	newDefaults, err = annotations.ModelDefaults("cr-defaults", c.podNamespace, c.store, c.store.ConfigMaps.Main.Annotations)
	if err != nil {
		logger.Errorf("Defaults config: %s", err)
	}
	if newDefaults == nil {
		newDefaults = &models.Defaults{}
		newDefaults.Name = constants.DefaultsSectionName
		for _, a := range c.annotations.Defaults(newDefaults) {
			logger.Error(a.Process(c.store, c.store.ConfigMaps.Main.Annotations))
		}
	}
	env.SetDefaults(newDefaults)
	newDefaults.ErrorFiles = defaults.ErrorFiles
	newDefaults.Name = constants.DefaultsSectionName
	diff := newDefaults.Diff(*defaults)
	if len(diff) != 0 {
		if err = c.haproxy.DefaultsPushConfiguration(*newDefaults); err != nil {
			logger.Error(err)
			return
		}
		instance.Reload("Defaults config updated: %+v", diff)
	}
}

// handleDefaultService configures HAProy default backend provided via cli param "default-backend-service"
func (c *HAProxyController) handleDefaultService() {
	var svc *service.Service
	namespace, name, err := common.GetK8sPath("default-backend-service", c.store.ConfigMaps.Main.Annotations)
	if err != nil {
		logger.Errorf("default service: %s", err)
	}
	if name == "" {
		return
	}
	ingressPath := &store.IngressPath{
		SvcNamespace:     namespace,
		SvcName:          name,
		IsDefaultBackend: true,
	}
	if svc, err = service.New(c.store, ingressPath, nil, false, nil, c.store.ConfigMaps.Main.Annotations); err == nil {
		err = svc.SetDefaultBackend(c.store, c.haproxy, []string{c.haproxy.FrontHTTP, c.haproxy.FrontHTTPS}, c.annotations)
	}
	if err != nil {
		logger.Errorf("default service: %s", err)
	}
}

func populateDefaultLocalBackendResources(k8sStore store.K8s, podNs string, defaultBackendPort int) error {
	controllerNs, ok := k8sStore.Namespaces[podNs]
	if !ok {
		return fmt.Errorf("controller namespace '%s' not found", podNs)
	}

	defaultLocalService := controllerNs.Services[store.DefaultLocalBackend]
	if defaultLocalService == nil {
		item := &store.Service{
			Namespace: podNs,
			Name:      store.DefaultLocalBackend,
			Status:    store.ADDED,

			Ports: []store.ServicePort{
				{
					Name:     "http",
					Protocol: "http",
					Port:     8765,
					Status:   store.ADDED,
				},
			},
		}
		k8sStore.EventService(controllerNs, item)
		logger.Debug("default backend event service processed")

		endpoints := &store.Endpoints{
			Namespace: podNs,
			Service:   store.DefaultLocalBackend,
			SliceName: store.DefaultLocalBackend,
			Status:    store.ADDED,
			Ports: map[string]*store.PortEndpoints{
				"http": {
					Port:      int64(defaultBackendPort),
					Addresses: map[string]struct{}{"127.0.0.1": {}},
				},
			},
		}
		k8sStore.EventEndpoints(controllerNs, endpoints, nil)
		logger.Debug("default backend event endpoints processed")
	} else {
		defaultLocalService.Annotations = k8sStore.ConfigMaps.Main.Annotations
	}
	return nil
}

func (c *HAProxyController) handleDefaultLocalService() {
	var (
		err error
		svc *service.Service
	)
	err = populateDefaultLocalBackendResources(c.store, c.podNamespace, c.osArgs.DefaultBackendPort)
	if err != nil {
		logger.Error(err)
		return
	}

	ingressPath := &store.IngressPath{
		SvcNamespace:     c.podNamespace,
		SvcName:          store.DefaultLocalBackend,
		IsDefaultBackend: true,
	}

	if svc, err = service.New(c.store, ingressPath, nil, false, nil, c.store.ConfigMaps.Main.Annotations); err == nil {
		err = svc.SetDefaultBackend(c.store, c.haproxy, []string{c.haproxy.FrontHTTP, c.haproxy.FrontHTTPS}, c.annotations)
	}
	if err != nil {
		logger.Errorf("default service: %s", err)
	}
}

// handleDefaultCert configures default/fallback HAProxy certificate to use for client HTTPS requests.
func (c *HAProxyController) handleDefaultCert() {
	secret, err := annotations.Secret("ssl-certificate", c.podNamespace, c.store, c.store.ConfigMaps.Main.Annotations)
	if err != nil {
		logger.Errorf("default certificate: %s", err)
		return
	}
	if secret == nil {
		return
	}
	_, err = c.haproxy.AddSecret(secret, certs.FT_DEFAULT_CERT)
	logger.Error(err)
}
