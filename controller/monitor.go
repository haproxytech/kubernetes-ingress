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
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func (c *HAProxyController) monitorChanges() {
	go c.SyncData()

	informersSynced := []cache.InformerSynced{}
	stop := make(chan struct{})

	for _, namespace := range c.getWhitelistedNamespaces() {
		factory := informers.NewSharedInformerFactoryWithOptions(c.k8s.API, c.OSArgs.CacheResyncPeriod, informers.WithNamespace(namespace))

		pi := factory.Core().V1().Endpoints().Informer()
		c.k8s.EventsEndpoints(c.eventChan, stop, pi)

		svci := factory.Core().V1().Services().Informer()
		c.k8s.EventsServices(c.eventChan, c.statusChan, stop, svci, c.PublishService)

		nsi := factory.Core().V1().Namespaces().Informer()
		c.k8s.EventsNamespaces(c.eventChan, stop, nsi)

		si := factory.Core().V1().Secrets().Informer()
		c.k8s.EventsSecrets(c.eventChan, stop, si)

		ci := factory.Core().V1().ConfigMaps().Informer()
		c.k8s.EventsConfigfMaps(c.eventChan, stop, ci)

		var ii, ici cache.SharedIndexInformer
		ii, ici = c.getIngressSharedInformers(factory)
		if ii == nil {
			logger.Panic("ingress resources not supported in this cluster")
		}
		c.k8s.EventsIngresses(c.eventChan, stop, ii)

		informersSynced = []cache.InformerSynced{pi.HasSynced, svci.HasSynced, nsi.HasSynced, ii.HasSynced, si.HasSynced, ci.HasSynced}

		if ici != nil {
			c.k8s.EventsIngressClass(c.eventChan, stop, ici)
			informersSynced = append(informersSynced, ici.HasSynced)
		}
	}

	if !cache.WaitForCacheSync(stop, informersSynced...) {
		logger.Panic("Caches are not populated due to an underlying error, cannot run the Ingress Controller")
	}

	syncPeriod := c.OSArgs.SyncPeriod
	logger.Debugf("Executing syncPeriod every %s", syncPeriod.String())
	for {
		time.Sleep(syncPeriod)
		c.eventChan <- SyncDataEvent{SyncType: COMMAND}
	}
}

// SyncData gets all kubernetes changes, aggregates them and apply to HAProxy.
// All the changes must come through this function
func (c *HAProxyController) SyncData() {
	hadChanges := false
	for job := range c.eventChan {
		ns := c.Store.GetNamespace(job.Namespace)
		change := false
		switch job.SyncType {
		case COMMAND:
			c.reload = c.auxCfgUpdated()
			if hadChanges || c.reload {
				c.updateHAProxy()
				hadChanges = false
				continue
			}
		case NAMESPACE:
			change = c.Store.EventNamespace(ns, job.Data.(*store.Namespace))
		case INGRESS:
			change = c.Store.EventIngress(ns, job.Data.(*store.Ingress), c.OSArgs.IngressClass)
		case INGRESS_CLASS:
			change = c.Store.EventIngressClass(job.Data.(*store.IngressClass))
		case ENDPOINTS:
			change = c.Store.EventEndpoints(ns, job.Data.(*store.Endpoints), c.Client.SyncBackendSrvs)
		case SERVICE:
			change = c.Store.EventService(ns, job.Data.(*store.Service))
		case CONFIGMAP:
			change = c.Store.EventConfigMap(ns, job.Data.(*store.ConfigMap))
		case SECRET:
			change = c.Store.EventSecret(ns, job.Data.(*store.Secret))
		}
		hadChanges = hadChanges || change
	}
}

func (c *HAProxyController) getIngressSharedInformers(factory informers.SharedInformerFactory) (ii, ici cache.SharedIndexInformer) {
	for i, apiGroup := range []string{"networking.k8s.io/v1", "networking.k8s.io/v1beta1", "extensions/v1beta1"} {
		resources, err := c.k8s.API.ServerResourcesForGroupVersion(apiGroup)
		if err != nil {
			continue
		}
		for _, rs := range resources.APIResources {
			if rs.Name == "ingresses" {
				switch i {
				case 0:
					ii = factory.Networking().V1().Ingresses().Informer()
				case 1:
					ii = factory.Networking().V1beta1().Ingresses().Informer()
				case 2:
					ii = factory.Extensions().V1beta1().Ingresses().Informer()
				}
				logger.Debugf("watching ingress resources of apiGroup %s:", apiGroup)
			}
			if rs.Name == "ingressclasses" {
				switch i {
				case 0:
					ici = factory.Networking().V1().IngressClasses().Informer()
				case 1:
					ici = factory.Networking().V1beta1().IngressClasses().Informer()
				}
			}
		}
		if ii != nil {
			break
		}
	}
	return ii, ici
}

func (c *HAProxyController) getWhitelistedNamespaces() []string {
	if len(c.Store.NamespacesAccess.Whitelist) == 0 {
		return []string{""}
	}
	// Add one because of potential whitelisting of configmap namespace
	namespaces := make([]string, len(c.Store.NamespacesAccess.Whitelist)+1)
	for ns := range c.Store.NamespacesAccess.Whitelist {
		namespaces = append(namespaces, ns)
	}
	cfgMapNS := c.OSArgs.ConfigMap.Namespace
	if _, ok := c.Store.NamespacesAccess.Whitelist[cfgMapNS]; !ok {
		namespaces = append(namespaces, cfgMapNS)
		logger.Warningf("configmap Namespace '%s' not whitelisted. Whitelisting it anyway", cfgMapNS)
	}
	logger.Infof("Whitelisted Namespaces: %s", namespaces)
	return namespaces
}

// auxCfgUpdate returns true if auxiliary HAProxy config file was updated, false otherwise.
func (c *HAProxyController) auxCfgUpdated() bool {
	info, errStat := os.Stat(c.Cfg.Env.AuxCFGFile)
	// File does not exist
	if errStat != nil {
		if c.AuxCfgModTime == 0 {
			return false
		}
		logger.Infof("Auxiliary HAProxy config '%s' removed", c.Cfg.Env.AuxCFGFile)
		c.AuxCfgModTime = 0
		c.Client.SetAuxCfgFile("")
		c.haproxyProcess.UseAuxFile(false)
		return true
	}
	// Check modification time
	modifTime := info.ModTime().Unix()
	if c.AuxCfgModTime == modifTime {
		return false
	}
	logger.Infof("Auxiliary HAProxy config '%s' updated", c.Cfg.Env.AuxCFGFile)
	c.AuxCfgModTime = modifTime
	c.Client.SetAuxCfgFile(c.Cfg.Env.AuxCFGFile)
	c.haproxyProcess.UseAuxFile(true)
	return true
}
