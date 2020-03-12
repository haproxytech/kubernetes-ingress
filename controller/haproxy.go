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
	"log"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// Sync HAProxy configuration
func (c *HAProxyController) updateHAProxy() error {
	needsReload := false

	err := c.apiStartTransaction()
	if err != nil {
		utils.LogErr(err)
		return err
	}
	defer func() {
		c.apiDisposeTransaction()
	}()
	reload := c.handleDefaultTimeouts()
	needsReload = needsReload || reload

	reload, err = c.handleMaxconn()
	utils.LogErr(err)
	needsReload = needsReload || reload

	reload, err = c.handleGlobalAnnotations()
	utils.LogErr(err)
	needsReload = needsReload || reload

	reload, err = c.handleDefaultService()
	utils.LogErr(err)
	needsReload = needsReload || reload

	usedCerts := map[string]struct{}{}

	for _, namespace := range c.cfg.Namespace {
		if !namespace.Relevant {
			continue
		}
		for _, ingress := range namespace.Ingresses {
			if c.cfg.PublishService != nil && ingress.Status != DELETED {
				utils.LogErr(c.k8s.UpdateIngressStatus(ingress, c.cfg.PublishService))
			}
			// handle Default Backend
			if ingress.DefaultBackend != nil {
				reload, err = c.handlePath(namespace, ingress, &IngressRule{}, ingress.DefaultBackend)
				utils.LogErr(err)
				needsReload = needsReload || reload
			}
			// handle Ingress rules
			for _, rule := range ingress.Rules {
				for _, path := range rule.Paths {
					reload, err = c.handlePath(namespace, ingress, rule, path)
					needsReload = needsReload || reload
					utils.LogErr(err)
				}
			}
			//handle certs
			ingressSecrets := map[string]struct{}{}
			for _, tls := range ingress.TLS {
				if _, ok := ingressSecrets[tls.SecretName.Value]; !ok {
					ingressSecrets[tls.SecretName.Value] = struct{}{}
					reload = c.handleTLSSecret(*ingress, *tls, usedCerts)
					needsReload = needsReload || reload
				}
			}

			utils.LogErr(c.handleRequestCapture(ingress))
			utils.LogErr(c.handleWhitelisting(ingress))
			utils.LogErr(c.handleHTTPRedirect(ingress))
		}
	}

	utils.LogErr(c.handleProxyProtocol())

	reload = c.handleDefaultCertificate(usedCerts)
	needsReload = needsReload || reload

	reload = c.handleHTTPS(usedCerts)
	needsReload = needsReload || reload

	reload, err = c.handleRateLimiting(c.cfg.HTTPS)
	if err != nil {
		return err
	}
	needsReload = needsReload || reload

	reload, err = c.cfg.MapFiles.Refresh()
	utils.LogErr(err)
	needsReload = needsReload || reload

	needsReload = c.RequestsHTTPRefresh() || needsReload

	needsReload = c.RequestsTCPRefresh() || needsReload

	reload, err = c.handleTCPServices()
	utils.LogErr(err)
	needsReload = needsReload || reload

	reload = c.refreshBackendSwitching()
	needsReload = needsReload || reload

	err = c.apiCommitTransaction()
	if err != nil {
		utils.LogErr(err)
		return err
	}
	c.cfg.Clean()
	if needsReload {
		if err := c.HAProxyReload(); err != nil {
			utils.LogErr(err)
		} else {
			log.Println("HAProxy reloaded")
		}
	}
	return nil
}

// handles defaultBackned configured via cli param "default-backend-service"
func (c *HAProxyController) handleDefaultService() (needsReload bool, err error) {
	needsReload = false
	dsvcData, _ := GetValueFromAnnotations("default-backend-service")
	dsvc := strings.Split(dsvcData.Value, "/")

	if len(dsvc) != 2 {
		return needsReload, fmt.Errorf("default service invalid data")
	}
	if dsvc[0] == "" || dsvc[1] == "" {
		return needsReload, nil
	}
	namespace, ok := c.cfg.Namespace[dsvc[0]]
	if !ok {
		return needsReload, fmt.Errorf("default service invalid namespace " + dsvc[0])
	}
	service, ok := namespace.Services[dsvc[1]]
	if !ok {
		return needsReload, fmt.Errorf("service '" + dsvc[1] + "' does not exist")
	}
	ingress := &Ingress{
		Namespace:   namespace.Name,
		Name:        "DefaultService",
		Annotations: MapStringW{},
		Rules:       map[string]*IngressRule{},
	}
	path := &IngressPath{
		ServiceName:      service.Name,
		ServicePortInt:   service.Ports[0].Port,
		IsDefaultBackend: true,
	}
	return c.handlePath(namespace, ingress, &IngressRule{}, path)
}
