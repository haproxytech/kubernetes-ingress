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

	corev1alpha1 "github.com/haproxytech/kubernetes-ingress/crs/api/core/v1alpha1"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
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
		case k8s.COMMAND:
			c.restart, c.reload = c.auxCfgManager()
			if hadChanges || c.reload || c.restart {
				c.updateHAProxy()
				hadChanges = false
				continue
			}
		case k8s.CR_GLOBAL:
			var data *corev1alpha1.Global
			if job.Data != nil {
				data = job.Data.(*corev1alpha1.Global)
			}
			change = c.store.EventGlobalCR(job.Namespace, job.Name, data)
		case k8s.CR_DEFAULTS:
			var data *corev1alpha1.Defaults
			if job.Data != nil {
				data = job.Data.(*corev1alpha1.Defaults)
			}
			change = c.store.EventDefaultsCR(job.Namespace, job.Name, data)
		case k8s.CR_BACKEND:
			var data *corev1alpha1.Backend
			if job.Data != nil {
				data = job.Data.(*corev1alpha1.Backend)
			}
			change = c.store.EventBackendCR(job.Namespace, job.Name, data)
		case k8s.NAMESPACE:
			change = c.store.EventNamespace(ns, job.Data.(*store.Namespace))
		case k8s.INGRESS:
			change = c.store.EventIngress(ns, job.Data.(*store.Ingress))
		case k8s.INGRESS_CLASS:
			change = c.store.EventIngressClass(job.Data.(*store.IngressClass))
		case k8s.ENDPOINTS:
			change = c.store.EventEndpoints(ns, job.Data.(*store.Endpoints), c.haproxy.SyncBackendSrvs)
		case k8s.SERVICE:
			change = c.store.EventService(ns, job.Data.(*store.Service))
		case k8s.CONFIGMAP:
			change = c.store.EventConfigMap(ns, job.Data.(*store.ConfigMap))
		case k8s.SECRET:
			change = c.store.EventSecret(ns, job.Data.(*store.Secret))
		case k8s.POD:
			change = c.store.EventPod(job.Data.(store.PodEvent))
		case k8s.PUBLISH_SERVICE:
			change = c.store.EventPublishService(ns, job.Data.(*store.Service))
		}
		hadChanges = hadChanges || change
		if job.EventProcessed != nil {
			close(job.EventProcessed)
		}

	}
}

// auxCfgManager returns restart or reload requirement based on state and transition of auxiliary configuration file.
func (c *HAProxyController) auxCfgManager() (restart, reload bool) {
	info, errStat := os.Stat(c.haproxy.AuxCFGFile)
	var (
		modifTime  int64
		auxCfgFile string = c.haproxy.AuxCFGFile
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
		if c.auxCfgModTime == 0 {
			restart = true
		} else {
			// File already exists,
			// already in command line parameters just need to reload for modifications.
			reload = true
		}
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
		logger.Infof("Auxiliary HAProxy config '%s' removed", c.haproxy.AuxCFGFile)
		// but existed so need to restart
		restart = true
		return
	}
	// File exists
	useAuxFile = true
	modifTime = info.ModTime().Unix()
	return
}
