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
)

func (c *HAProxyController) timeFromAnnotation(name string) (duration time.Duration) {
	d, err := GetValueFromAnnotations(name)
	if err != nil {
		c.Logger.Panic(err)
	}
	duration, _ = time.ParseDuration(d.Value)

	return
}

func (c *HAProxyController) monitorChanges() {

	configMapReceivedAndProcessed := make(chan bool)
	syncPeriod := c.timeFromAnnotation("sync-period")
	c.Logger.Debugf("Executing syncPeriod every %s", syncPeriod.String())
	go c.SyncData(c.eventChan, configMapReceivedAndProcessed)

	stop := make(chan struct{})
	factory := informers.NewSharedInformerFactory(c.k8s.API, c.timeFromAnnotation("cache-resync-period"))

	endpointsChan := make(chan *Endpoints, 100)
	pi := factory.Core().V1().Endpoints().Informer()
	c.k8s.EventsEndpoints(endpointsChan, stop, pi)

	svci := factory.Core().V1().Services().Informer()
	svcChan := make(chan *Service, 100)
	c.k8s.EventsServices(svcChan, stop, svci, c.cfg.PublishService)

	nsi := factory.Core().V1().Namespaces().Informer()
	nsChan := make(chan *Namespace, 10)
	c.k8s.EventsNamespaces(nsChan, stop, nsi)

	ii := factory.Extensions().V1beta1().Ingresses().Informer()
	ingChan := make(chan *Ingress, 10)
	c.k8s.EventsIngresses(ingChan, stop, ii)

	ci := factory.Core().V1().ConfigMaps().Informer()
	cfgChan := make(chan *ConfigMap, 10)
	c.k8s.EventsConfigfMaps(cfgChan, stop, ci)

	si := factory.Core().V1().Secrets().Informer()
	secretChan := make(chan *Secret, 10)
	c.k8s.EventsSecrets(secretChan, stop, si)

	if !cache.WaitForCacheSync(stop, pi.HasSynced, svci.HasSynced, nsi.HasSynced, ii.HasSynced, ci.HasSynced, si.HasSynced) {
		c.Logger.Panic("Caches are not populated due to an underlying error, cannot run the Ingress Controller")
	}

	// Buffering events so they are handled after configMap is processed
	var eventsIngress, eventsEndpoints, eventsServices []SyncDataEvent
	var configMapOk bool
	if c.osArgs.ConfigMap.Name == "" {
		configMapOk = true
		//since we don't have configmap and everywhere in code we expect one we need to create empty one
		c.cfg.ConfigMap = &ConfigMap{
			Annotations: MapStringW{},
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
			c.Logger.Debug("Configmap processed")
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

//SyncData gets all kubernetes changes, aggregates them and apply to HAProxy.
//All the changes must come through this function
func (c *HAProxyController) SyncData(jobChan <-chan SyncDataEvent, chConfigMapReceivedAndProcessed chan bool) {
	hadChanges := false
	for job := range jobChan {
		ns := c.cfg.GetNamespace(job.Namespace)
		change := false
		switch job.SyncType {
		case COMMAND:
			if hadChanges {
				if err := c.updateHAProxy(); err != nil {
					c.Logger.Error(err)
					continue
				}
				hadChanges = false
				continue
			}
		case NAMESPACE:
			change = c.eventNamespace(ns, job.Data.(*Namespace))
		case INGRESS:
			change = c.eventIngress(ns, job.Data.(*Ingress))
		case ENDPOINTS:
			change = c.eventEndpoints(ns, job.Data.(*Endpoints))
		case SERVICE:
			change = c.eventService(ns, job.Data.(*Service))
		case CONFIGMAP:
			change = c.eventConfigMap(ns, job.Data.(*ConfigMap), chConfigMapReceivedAndProcessed)
		case SECRET:
			change = c.eventSecret(ns, job.Data.(*Secret))
		}
		hadChanges = hadChanges || change
	}
}
