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
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

func (c *HAProxyController) timeFromAnnotation(name string) (duration time.Duration) {
	d, err := c.Store.GetValueFromAnnotations(name)
	if err != nil {
		logger.Panic(err)
	}
	duration, _ = time.ParseDuration(d.Value)

	return
}

func (c *HAProxyController) monitorChanges() {
	go c.SyncData()

	informersSynced := []cache.InformerSynced{}
	stop := make(chan struct{})

	for _, namespace := range c.getWhitelistedNamespaces() {
		factory := informers.NewSharedInformerFactoryWithOptions(c.k8s.API, c.timeFromAnnotation("cache-resync-period"), informers.WithNamespace(namespace))

		pi := factory.Core().V1().Endpoints().Informer()
		c.k8s.EventsEndpoints(c.eventChan, stop, pi)

		svci := factory.Core().V1().Services().Informer()
		c.k8s.EventsServices(c.eventChan, stop, svci, c.PublishService)

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

	syncPeriod := c.timeFromAnnotation("sync-period")
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
	configMapArgs := c.getConfigMapArgs()
	for job := range c.eventChan {
		ns := c.Store.GetNamespace(job.Namespace)
		change := false
		switch job.SyncType {
		case COMMAND:
			if hadChanges {
				c.updateHAProxy()
				hadChanges = false
				continue
			}
		case NAMESPACE:
			change = c.Store.EventNamespace(ns, job.Data.(*store.Namespace))
		case INGRESS:
			change = c.Store.EventIngress(ns, job.Data.(*store.Ingress), c.IngressClass)
		case INGRESS_CLASS:
			change = c.Store.EventIngressClass(job.Data.(*store.IngressClass))
		case ENDPOINTS:
			change = c.Store.EventEndpoints(ns, job.Data.(*store.Endpoints), c.updateHAProxySrvs)
		case SERVICE:
			change = c.Store.EventService(ns, job.Data.(*store.Service))
		case CONFIGMAP:
			change, _ = c.Store.EventConfigMap(ns, job.Data.(*store.ConfigMap), configMapArgs)
		case SECRET:
			change = c.Store.EventSecret(ns, job.Data.(*store.Secret))
		}
		hadChanges = hadChanges || change
	}
}

// updateHAProxySrvs dynamically update (via runtime socket) HAProxy backend servers with modifed Addresses
func (c *HAProxyController) updateHAProxySrvs(oldEndpoints, newEndpoints *store.PortEndpoints) {
	if oldEndpoints.BackendName == "" {
		return
	}
	newEndpoints.HAProxySrvs = oldEndpoints.HAProxySrvs
	newEndpoints.BackendName = oldEndpoints.BackendName
	haproxySrvs := newEndpoints.HAProxySrvs
	newAddresses := newEndpoints.AddrNew
	// Disable stale entries from HAProxySrvs
	// and provide list of Disabled Srvs
	var disabled []*store.HAProxySrv
	for i, srv := range haproxySrvs {
		if _, ok := newAddresses[srv.Address]; ok {
			delete(newAddresses, srv.Address)
		} else {
			haproxySrvs[i].Address = ""
			haproxySrvs[i].Modified = true
			disabled = append(disabled, srv)
		}
	}

	// Configure new Addresses in available HAProxySrvs
	for newAddr := range newAddresses {
		if len(disabled) == 0 {
			break
		}
		disabled[0].Address = newAddr
		disabled[0].Modified = true
		disabled = disabled[1:]
		delete(newAddresses, newAddr)
	}
	// Dynamically updates HAProxy backend servers  with HAProxySrvs content
	var addrErr, stateErr error
	for _, srv := range haproxySrvs {
		if !srv.Modified {
			continue
		}
		if srv.Address == "" {
			logger.Tracef("server '%s/%s' changed status to %v", newEndpoints.BackendName, srv.Name, "maint")
			addrErr = c.Client.SetServerAddr(newEndpoints.BackendName, srv.Name, "127.0.0.1", 0)
			stateErr = c.Client.SetServerState(newEndpoints.BackendName, srv.Name, "maint")
		} else {
			logger.Tracef("server '%s/%s' changed status to %v", newEndpoints.BackendName, srv.Name, "ready")
			addrErr = c.Client.SetServerAddr(newEndpoints.BackendName, srv.Name, srv.Address, 0)
			stateErr = c.Client.SetServerState(newEndpoints.BackendName, srv.Name, "ready")
		}
		if addrErr != nil || stateErr != nil {
			newEndpoints.DynUpdateFailed = true
			logger.Error(addrErr)
			logger.Error(stateErr)
		}
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
	namespaces := make([]string, len(c.Store.NamespacesAccess.Whitelist))
	for ns := range c.Store.NamespacesAccess.Whitelist {
		namespaces = append(namespaces, ns)
	}
	cfgMapNS := c.osArgs.ConfigMap.Namespace
	if _, ok := c.Store.NamespacesAccess.Whitelist[cfgMapNS]; !ok {
		namespaces = []string{cfgMapNS}
		logger.Warningf("configmap Namespace '%s' not whitelisted. Whitelisting it anyway", cfgMapNS)
	}
	logger.Infof("Whitelisted Namespaces: %s", namespaces)
	return namespaces
}

func (c *HAProxyController) getConfigMapArgs() map[string]utils.NamespaceValue {
	return map[string]utils.NamespaceValue{
		Main: {
			Namespace: c.osArgs.ConfigMap.Namespace,
			Name:      c.osArgs.ConfigMap.Name,
		},
		TCPServices: {
			Namespace: c.osArgs.ConfigMapTCPServices.Namespace,
			Name:      c.osArgs.ConfigMapTCPServices.Name,
		},
		Errorfiles: {
			Namespace: c.osArgs.ConfigMapErrorfiles.Namespace,
			Name:      c.osArgs.ConfigMapErrorfiles.Name,
		},
	}
}
