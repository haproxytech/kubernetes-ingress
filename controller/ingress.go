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
	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/route"
	"github.com/haproxytech/kubernetes-ingress/controller/service"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// igClassIsSupported verifies if the IngressClass matches the ControllerClass
// and in such case returns true otherwise false
//
// According to https://github.com/kubernetes/api/blob/master/networking/v1/types.go#L257
// ingress.class annotation should have precedence over the IngressClass mechanism implemented
// in "networking.k8s.io".
func (c *HAProxyController) igClassIsSupported(ingress *store.Ingress) bool {
	var igClassAnn string
	var igClass *store.IngressClass
	if ann, _ := c.Store.GetValueFromAnnotations("ingress.class", ingress.Annotations); ann != nil {
		igClassAnn = ann.Value
	}

	// If ingress class is unassigned and the controller is controlling any resource without explicit ingress class then support it.
	if igClassAnn == "" && c.EmptyIngressClass {
		return true
	}

	if igClassAnn == "" || igClassAnn != c.IngressClass {
		igClass = c.Store.IngressClasses[ingress.Class]
		if igClass != nil && igClass.Status != DELETED && igClass.Controller == CONTROLLER_CLASS {
			// Corresponding IngresClass was updated so Ingress resource should be re-processed
			// This is particularly important if the Ingress was skipped due to mismatching ingrssClass
			if igClass.Status != EMPTY {
				ingress.Status = MODIFIED
			}
			return true
		}
	}
	if igClassAnn == c.IngressClass {
		return true
	}
	return false
}

func (c *HAProxyController) handleIngressPath(ingress *store.Ingress, host string, path *store.IngressPath) (reload bool, err error) {
	sslPassthrough := c.sslPassthroughEnabled(ingress, path)
	svc, err := service.NewCtx(c.Store, ingress, path, sslPassthrough)
	if err != nil {
		return
	}
	if svc.GetStatus() == DELETED {
		return
	}
	backendReload, backendName, err := svc.HandleBackend(c.Client, c.Store)
	if err != nil {
		return
	}
	err = route.AddHostPathRoute(route.Route{
		Host:           host,
		Path:           path,
		HAProxyRules:   c.cfg.HAProxyRules.GetIngressRuleIDs(ingress.Namespace + "-" + ingress.Name),
		BackendName:    backendName,
		SSLPassthrough: sslPassthrough,
	}, c.cfg.MapFiles)
	if err != nil {
		return
	}
	c.cfg.ActiveBackends[backendName] = struct{}{}
	endpointsReload := svc.HandleEndpoints(c.Client, c.Store, c.cfg.Certificates)
	reload = backendReload || endpointsReload
	return
}

func (c *HAProxyController) setDefaultService(ingress *store.Ingress, frontends []string) (reload bool, err error) {
	var frontend models.Frontend
	var ftReload bool
	frontend, err = c.Client.FrontendGet(frontends[0])
	if err != nil {
		return
	}
	tcpService := false
	if frontend.Mode == "tcp" {
		tcpService = true
	}
	svc, err := service.NewCtx(c.Store, ingress, ingress.DefaultBackend, tcpService)
	if err != nil {
		return
	}
	if svc.GetStatus() == DELETED {
		return
	}
	bdReload, backendName, err := svc.HandleBackend(c.Client, c.Store)
	if err != nil {
		return
	}
	if frontend.DefaultBackend != backendName {
		if frontend.Name == FrontendHTTP {
			logger.Infof("Setting http default backend to '%s'", backendName)
		}
		for _, frontendName := range frontends {
			frontend, _ := c.Client.FrontendGet(frontendName)
			frontend.DefaultBackend = backendName
			err = c.Client.FrontendEdit(frontend)
			if err != nil {
				return
			}
			ftReload = true
			logger.Debugf("Setting '%s' default backend to '%s'", frontendName, backendName)
			c.cfg.ActiveBackends[backendName] = struct{}{}
		}
	}
	endpointsReload := svc.HandleEndpoints(c.Client, c.Store, c.cfg.Certificates)
	reload = bdReload || ftReload || endpointsReload
	return reload, err
}

func (c *HAProxyController) sslPassthroughEnabled(ingress *store.Ingress, path *store.IngressPath) bool {
	var annSSLPassthrough *store.StringW
	var service *store.Service
	ok := false
	if path != nil {
		service, ok = c.Store.Namespaces[ingress.Namespace].Services[path.SvcName]
	}
	if ok {
		annSSLPassthrough, _ = c.Store.GetValueFromAnnotations("ssl-passthrough", service.Annotations, ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	} else {
		annSSLPassthrough, _ = c.Store.GetValueFromAnnotations("ssl-passthrough", ingress.Annotations, c.Store.ConfigMaps.Main.Annotations)
	}
	enabled, err := utils.GetBoolValue(annSSLPassthrough.Value, "ssl-passthrough")
	if err != nil {
		logger.Errorf("ssl-passthrough annotation: %s", err)
		return false
	}
	if annSSLPassthrough.Status == DELETED {
		return false
	}
	if enabled {
		c.cfg.SSLPassthrough = true
		return true
	}
	return false
}
