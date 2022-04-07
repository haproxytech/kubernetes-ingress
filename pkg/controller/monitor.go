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
	"strconv"
	"time"

	cr "github.com/haproxytech/kubernetes-ingress/pkg/customresource"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func (c *HAProxyController) monitorChanges() {
	go c.SyncData()

	informersSynced := []cache.InformerSynced{}
	stop := make(chan struct{})
	epMirror := c.endpointsMirroring()
	c.crManager = cr.NewCRManager(&c.store, c.k8s.RestConfig, c.osArgs.CacheResyncPeriod, c.eventChan, stop)

	c.k8s.EventPods(c.podNamespace, c.podPrefix, c.osArgs.CacheResyncPeriod, c.eventChan)

	for _, namespace := range c.getWhitelistedNamespaces() {
		factory := informers.NewSharedInformerFactoryWithOptions(c.k8s.API, c.osArgs.CacheResyncPeriod, informers.WithNamespace(namespace))

		// Core.V1 Resources
		svci := factory.Core().V1().Services().Informer()
		c.k8s.EventsServices(c.eventChan, c.ingressChan, stop, svci, c.publishService)

		nsi := factory.Core().V1().Namespaces().Informer()
		c.k8s.EventsNamespaces(c.eventChan, stop, nsi)

		si := factory.Core().V1().Secrets().Informer()
		c.k8s.EventsSecrets(c.eventChan, stop, si)

		ci := factory.Core().V1().ConfigMaps().Informer()
		c.k8s.EventsConfigfMaps(c.eventChan, stop, ci)

		informersSynced = append(informersSynced, svci.HasSynced, nsi.HasSynced, si.HasSynced, ci.HasSynced)

		// Ingress and IngressClass Resources
		ii, ici := c.getIngressSharedInformers(factory)
		if ii == nil {
			logger.Panic("Ingress Resource not supported in this cluster")
		}
		c.k8s.EventsIngresses(c.eventChan, stop, ii)
		informersSynced = append(informersSynced, ii.HasSynced)
		if ici != nil {
			c.k8s.EventsIngressClass(c.eventChan, stop, ici)
			informersSynced = append(informersSynced, ici.HasSynced)
		}

		// Endpoints and EndpointSlices Resources
		epsi := c.getEndpointSlicesSharedInformer(factory)
		if epsi != nil {
			c.k8s.EventsEndpointSlices(c.eventChan, stop, epsi)
			informersSynced = append(informersSynced, epsi.HasSynced)
		}
		if epsi == nil || !epMirror {
			epi := factory.Core().V1().Endpoints().Informer()
			c.k8s.EventsEndpoints(c.eventChan, stop, epi)
			informersSynced = append(informersSynced, epi.HasSynced)
		}

		// Custom Resources
		informersSynced = append(informersSynced, c.crManager.RunInformers(namespace)...)
	}

	if !cache.WaitForCacheSync(stop, informersSynced...) {
		logger.Panic("Caches are not populated due to an underlying error, cannot run the Ingress Controller")
	}

	syncPeriod := c.osArgs.SyncPeriod
	logger.Debugf("Executing syncPeriod every %s", syncPeriod.String())
	for {
		time.Sleep(syncPeriod)
		c.eventChan <- k8s.SyncDataEvent{SyncType: k8s.COMMAND}
	}
}

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
		case k8s.CUSTOM_RESOURCE:
			change = c.crManager.EventCustomResource(job)
		case k8s.NAMESPACE:
			change = c.store.EventNamespace(ns, job.Data.(*store.Namespace))
		case k8s.INGRESS:
			change = c.store.EventIngress(ns, job.Data.(*store.Ingress))
		case k8s.INGRESS_CLASS:
			change = c.store.EventIngressClass(job.Data.(*store.IngressClass))
		case k8s.ENDPOINTS:
			change = c.store.EventEndpoints(ns, job.Data.(*store.Endpoints), c.client.SyncBackendSrvs)
		case k8s.SERVICE:
			change = c.store.EventService(ns, job.Data.(*store.Service))
		case k8s.CONFIGMAP:
			change = c.store.EventConfigMap(ns, job.Data.(*store.ConfigMap))
		case k8s.SECRET:
			change = c.store.EventSecret(ns, job.Data.(*store.Secret))
		case k8s.POD:
			change = c.store.EventPod(job.Data.(store.PodEvent))
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

func (c *HAProxyController) getEndpointSlicesSharedInformer(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	for i, apiGroup := range []string{"discovery.k8s.io/v1", "discovery.k8s.io/v1beta1"} {
		resources, err := c.k8s.API.ServerResourcesForGroupVersion(apiGroup)
		if err != nil {
			continue
		}

		for _, rs := range resources.APIResources {
			if rs.Name == "endpointslices" {
				switch i {
				case 0:
					logger.Debug("Using discovery.k8s.io/v1 endpointslices")
					return factory.Discovery().V1().EndpointSlices().Informer()

				case 1:
					logger.Debug("Using discovery.k8s.io/v1beta1 endpointslices")
					return factory.Discovery().V1beta1().EndpointSlices().Informer()
				}
			}
		}
	}
	return nil
}

func (c *HAProxyController) getWhitelistedNamespaces() []string {
	if len(c.store.NamespacesAccess.Whitelist) == 0 {
		return []string{""}
	}
	// Add one because of potential whitelisting of configmap namespace
	namespaces := []string{}
	for ns := range c.store.NamespacesAccess.Whitelist {
		namespaces = append(namespaces, ns)
	}
	cfgMapNS := c.osArgs.ConfigMap.Namespace
	if _, ok := c.store.NamespacesAccess.Whitelist[cfgMapNS]; !ok {
		namespaces = append(namespaces, cfgMapNS)
		logger.Warningf("configmap Namespace '%s' not whitelisted. Whitelisting it anyway", cfgMapNS)
	}
	logger.Infof("Whitelisted Namespaces: %s", namespaces)
	return namespaces
}

// if EndpointSliceMirroring is supported we can just watch endpointSlices
// Ref: https://github.com/kubernetes/enhancements/tree/master/keps/sig-network/0752-endpointslices#endpointslicemirroring-controller
func (c *HAProxyController) endpointsMirroring() bool {
	var major, minor int
	var err error
	version, _ := c.k8s.API.ServerVersion()
	if version == nil {
		return false
	}
	major, err = strconv.Atoi(version.Major)
	if err != nil {
		return false
	}
	minor, err = strconv.Atoi(version.Minor)
	if err != nil {
		return false
	}
	if major == 1 && minor < 19 {
		return false
	}
	return true
}

// auxCfgManager returns restart or reload requirement based on state and transition of auxiliary configuration file.
func (c *HAProxyController) auxCfgManager() (restart, reload bool) {
	info, errStat := os.Stat(c.cfg.Env.AuxCFGFile)
	var (
		modifTime  int64
		auxCfgFile string = c.cfg.Env.AuxCFGFile
		useAuxFile bool
	)

	defer func() {
		// Nothing changed
		if c.auxCfgModTime == modifTime {
			return
		}
		// Apply decisions
		c.client.SetAuxCfgFile(auxCfgFile)
		c.haproxyProcess.UseAuxFile(useAuxFile)
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
		logger.Infof("Auxiliary HAProxy config '%s' removed", c.cfg.Env.AuxCFGFile)
		// but existed so need to restart
		restart = true
		return
	}
	// File exists
	useAuxFile = true
	modifTime = info.ModTime().Unix()
	return
}
