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

	configMapReceivedAndProcessed := make(chan bool)
	syncPeriod := c.timeFromAnnotation("sync-period")
	logger.Debugf("Executing syncPeriod every %s", syncPeriod.String())
	go c.SyncData(configMapReceivedAndProcessed)

	informersSynced := []cache.InformerSynced{}
	stop := make(chan struct{})
	endpointsChan := make(chan *store.Endpoints, 100)
	svcChan := make(chan *store.Service, 100)
	nsChan := make(chan *store.Namespace, 10)
	ingChan := make(chan *store.Ingress, 10)
	ingClassChan := make(chan *store.IngressClass, 10)
	cfgChan := make(chan *store.ConfigMap, 10)
	secretChan := make(chan *store.Secret, 10)

	var namespaces []string
	var ns string
	if len(c.Store.NamespacesAccess.Whitelist) == 0 {
		namespaces = []string{""}
	} else {
		for ns = range c.Store.NamespacesAccess.Whitelist {
			namespaces = append(namespaces, ns)
		}
		// Make sure that configmap namespace is whitelisted
		for _, ns = range namespaces {
			if ns == c.osArgs.ConfigMap.Namespace {
				break
			}
		}
		if ns != c.osArgs.ConfigMap.Namespace {
			namespaces = append(namespaces, c.osArgs.ConfigMap.Namespace)
		}
		logger.Infof("Whitelisted Namespaces: %s", namespaces)
	}

	for _, namespace := range namespaces {
		factory := informers.NewSharedInformerFactoryWithOptions(c.k8s.API, c.timeFromAnnotation("cache-resync-period"), informers.WithNamespace(namespace))

		pi := factory.Core().V1().Endpoints().Informer()
		informersSynced = append(informersSynced, pi.HasSynced)
		c.k8s.EventsEndpoints(endpointsChan, stop, pi)

		svci := factory.Core().V1().Services().Informer()
		informersSynced = append(informersSynced, svci.HasSynced)
		c.k8s.EventsServices(svcChan, stop, svci, c.PublishService)

		nsi := factory.Core().V1().Namespaces().Informer()
		informersSynced = append(informersSynced, nsi.HasSynced)
		c.k8s.EventsNamespaces(nsChan, stop, nsi)

		var ii, ici cache.SharedIndexInformer
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
					informersSynced = append(informersSynced, ii.HasSynced)
					c.k8s.EventsIngresses(ingChan, stop, ii)
				}
				if rs.Name == "ingressclasses" {
					switch i {
					case 0:
						ici = factory.Networking().V1().IngressClasses().Informer()
					case 1:
						ici = factory.Networking().V1beta1().IngressClasses().Informer()
					}
					informersSynced = append(informersSynced, ici.HasSynced)
					c.k8s.EventsIngressClass(ingClassChan, stop, ici)
				}
			}
		}
		if ii == nil {
			logger.Panic("ingress resources not supported in this cluster")
		}

		ci := factory.Core().V1().ConfigMaps().Informer()
		informersSynced = append(informersSynced, ci.HasSynced)
		c.k8s.EventsConfigfMaps(cfgChan, stop, ci)

		si := factory.Core().V1().Secrets().Informer()
		informersSynced = append(informersSynced, si.HasSynced)
		c.k8s.EventsSecrets(secretChan, stop, si)
	}

	if !cache.WaitForCacheSync(stop, informersSynced...) {
		logger.Panic("Caches are not populated due to an underlying error, cannot run the Ingress Controller")
	}

	// Buffering events so they are handled after configMap is processed
	var eventsIngress, eventsEndpoints, eventsServices []SyncDataEvent
	var configMapOk bool
	if c.osArgs.ConfigMap.Name == "" {
		configMapOk = true
		// since we don't have configmap and everywhere in code we expect one we need to create empty one
		c.Store.ConfigMaps[Main] = &store.ConfigMap{
			Annotations: store.MapStringW{},
		}
	} else {
		eventsIngress = []SyncDataEvent{}
		eventsEndpoints = []SyncDataEvent{}
		eventsServices = []SyncDataEvent{}
		configMapOk = false
	}

	for {
		select {
		case <-configMapReceivedAndProcessed:
			for _, event := range eventsIngress {
				c.eventChan <- event
			}
			for _, event := range eventsEndpoints {
				c.eventChan <- event
			}
			for _, event := range eventsServices {
				c.eventChan <- event
			}
			eventsIngress = []SyncDataEvent{}
			eventsEndpoints = []SyncDataEvent{}
			eventsServices = []SyncDataEvent{}
			configMapOk = true
			logger.Printf("Configmap processed\n")
			time.Sleep(1 * time.Millisecond)
		case item := <-cfgChan:
			c.eventChan <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: item.Namespace, Data: item}
		case item := <-nsChan:
			event := SyncDataEvent{SyncType: NAMESPACE, Namespace: item.Name, Data: item}
			c.eventChan <- event
		case item := <-endpointsChan:
			event := SyncDataEvent{SyncType: ENDPOINTS, Namespace: item.Namespace, Data: item}
			if configMapOk {
				c.eventChan <- event
			} else {
				eventsEndpoints = append(eventsEndpoints, event)
			}
		case item := <-svcChan:
			event := SyncDataEvent{SyncType: SERVICE, Namespace: item.Namespace, Data: item}
			if configMapOk {
				c.eventChan <- event
			} else {
				eventsServices = append(eventsServices, event)
			}
		case item := <-ingChan:
			event := SyncDataEvent{SyncType: INGRESS, Namespace: item.Namespace, Data: item}
			if configMapOk {
				c.eventChan <- event
			} else {
				eventsIngress = append(eventsIngress, event)
			}
		case item := <-ingClassChan:
			event := SyncDataEvent{SyncType: INGRESS_CLASS, Data: item}
			if configMapOk {
				c.eventChan <- event
			} else {
				eventsIngress = append(eventsIngress, event)
			}
		case item := <-secretChan:
			event := SyncDataEvent{SyncType: SECRET, Namespace: item.Namespace, Data: item}
			c.eventChan <- event
		case <-time.After(syncPeriod):
			if configMapOk && len(eventsIngress) == 0 && len(eventsServices) == 0 && len(eventsEndpoints) == 0 {
				c.eventChan <- SyncDataEvent{SyncType: COMMAND}
			}
		}
	}
}

// SyncData gets all kubernetes changes, aggregates them and apply to HAProxy.
// All the changes must come through this function
func (c *HAProxyController) SyncData(chConfigMapReceivedAndProcessed chan bool) {
	hadChanges := false
	var cm string
	ConfigMapsArgs := map[string]utils.NamespaceValue{
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
			change, cm = c.Store.EventConfigMap(ns, job.Data.(*store.ConfigMap), ConfigMapsArgs)
			if cm == Main && c.Store.ConfigMaps[Main].Status == ADDED {
				chConfigMapReceivedAndProcessed <- true
			}
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
			logger.Debugf("server '%s/%s' changed status to %v", newEndpoints.BackendName, srv.Name, "maint")
			addrErr = c.Client.SetServerAddr(newEndpoints.BackendName, srv.Name, "127.0.0.1", 0)
			stateErr = c.Client.SetServerState(newEndpoints.BackendName, srv.Name, "maint")
		} else {
			logger.Debugf("server '%s/%s' changed status to %v", newEndpoints.BackendName, srv.Name, "ready")
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
