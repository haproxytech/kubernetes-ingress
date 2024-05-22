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
	"os"

	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// SyncData gets all kubernetes changes, aggregates them and apply to HAProxy.
// All the changes must come through this function
func (c *HAProxyController) SyncData() {
	hadChanges := false
	for job := range c.eventChan {
		ns := c.store.GetNamespace(job.Namespace)
		change := false
		switch job.SyncType {
		case k8ssync.COMMAND:
			c.auxCfgManager()
			// create a NeedAction function.
			if hadChanges || instance.NeedAction() {
				c.updateHAProxy()
				hadChanges = false
				continue
			}
		case k8ssync.CR_GLOBAL:
			var data *v1.Global
			if job.Data != nil {
				data = job.Data.(*v1.Global) //nolint:forcetypeassert
			}
			change = c.store.EventGlobalCR(job.Namespace, job.Name, data)
		case k8ssync.CR_DEFAULTS:
			var data *v1.Defaults
			if job.Data != nil {
				data = job.Data.(*v1.Defaults) //nolint:forcetypeassert
			}
			change = c.store.EventDefaultsCR(job.Namespace, job.Name, data)
		case k8ssync.CR_BACKEND:
			var data *v1.Backend
			if job.Data != nil {
				data = job.Data.(*v1.Backend) //nolint:forcetypeassert
			}
			change = c.store.EventBackendCR(job.Namespace, job.Name, data)
		case k8ssync.NAMESPACE:
			change = c.store.EventNamespace(ns, job.Data.(*store.Namespace)) //nolint:forcetypeassert
		case k8ssync.INGRESS:
			change = c.store.EventIngress(ns, job.Data.(*store.Ingress)) //nolint:forcetypeassert
		case k8ssync.INGRESS_CLASS:
			change = c.store.EventIngressClass(job.Data.(*store.IngressClass)) //nolint:forcetypeassert
		case k8ssync.ENDPOINTS:
			change = c.store.EventEndpoints(ns, job.Data.(*store.Endpoints), c.haproxy.SyncBackendSrvs) //nolint:forcetypeassert
		case k8ssync.SERVICE:
			change = c.store.EventService(ns, job.Data.(*store.Service)) //nolint:forcetypeassert
		case k8ssync.CONFIGMAP:
			change = c.store.EventConfigMap(ns, job.Data.(*store.ConfigMap)) //nolint:forcetypeassert
		case k8ssync.SECRET:
			change = c.store.EventSecret(ns, job.Data.(*store.Secret)) //nolint:forcetypeassert
		case k8ssync.POD:
			change = c.store.EventPod(job.Data.(store.PodEvent)) //nolint:forcetypeassert
		case k8ssync.PUBLISH_SERVICE:
			change = c.store.EventPublishService(ns, job.Data.(*store.Service)) //nolint:forcetypeassert
		case k8ssync.GATEWAYCLASS:
			change = c.store.EventGatewayClass(job.Data.(*store.GatewayClass))
		case k8ssync.GATEWAY:
			change = c.store.EventGateway(ns, job.Data.(*store.Gateway))
		case k8ssync.TCPROUTE:
			change = c.store.EventTCPRoute(ns, job.Data.(*store.TCPRoute))
		case k8ssync.REFERENCEGRANT:
			change = c.store.EventReferenceGrant(ns, job.Data.(*store.ReferenceGrant))
		case k8ssync.CR_TCP:
			var data *store.TCPs
			if job.Data != nil {
				data = job.Data.(*store.TCPs) //nolint:forcetypeassert
			}
			change = c.store.EventTCPCR(job.Namespace, job.Name, data)
		}
		hadChanges = hadChanges || change
		if job.EventProcessed != nil {
			close(job.EventProcessed)
		}
	}
}

// auxCfgManager returns restart or reload requirement based on state and transition of auxiliary configuration file.
func (c *HAProxyController) auxCfgManager() {
	info, errStat := os.Stat(c.haproxy.AuxCFGFile)
	var (
		modifTime  int64
		auxCfgFile = c.haproxy.AuxCFGFile
		useAuxFile bool
	)

	defer func() {
		// Nothing changed
		if c.auxCfgModTime == modifTime {
			return
		}
		// Apply decisions
		c.haproxy.SetAuxCfgFile(auxCfgFile)
		c.haproxy.UseAuxFile(useAuxFile)
		// The file exists now  (modifTime !=0 otherwise nothing changed case).
		instance.RestartIf(c.auxCfgModTime == 0, "auxiliary configuration file created")
		instance.ReloadIf(c.auxCfgModTime != 0, "auxiliary configuration file modified")
		c.auxCfgModTime = modifTime
		if c.auxCfgModTime != 0 {
			logger.Infof("Auxiliary HAProxy config '%s' updated", auxCfgFile)
		}
	}()

	// File does not exist
	if errStat != nil {
		// nullify it
		auxCfgFile = ""
		if c.auxCfgModTime == 0 {
			// never existed before
			return
		}
		instance.Restart("Auxiliary HAProxy config '%s' removed", c.haproxy.AuxCFGFile)
		return
	}
	// File exists
	useAuxFile = true
	modifTime = info.ModTime().Unix()
}
