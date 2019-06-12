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

package main

import (
	"log"
	"time"
)

func (c *HAProxyController) monitorChanges() {

	configMapReceivedAndProcessed := make(chan bool)
	syncEveryNSeconds := 5
	go c.SyncData(c.eventChan, configMapReceivedAndProcessed)

	stop := make(chan struct{})

	podChan := make(chan *Pod, 100)
	c.k8s.EventsPods(podChan, stop)

	svcChan := make(chan *Service, 100)
	c.k8s.EventsServices(svcChan, stop)

	nsChan := make(chan *Namespace, 10)
	c.k8s.EventsNamespaces(nsChan, stop)

	ingChan := make(chan *Ingress, 10)
	c.k8s.EventsIngresses(ingChan, stop)

	cfgChan := make(chan *ConfigMap, 10)
	c.k8s.EventsConfigfMaps(cfgChan, stop)

	secretChan := make(chan *Secret, 10)
	c.k8s.EventsSecrets(secretChan, stop)

	eventsIngress := []SyncDataEvent{}
	eventsServices := []SyncDataEvent{}
	eventsPods := []SyncDataEvent{}
	configMapOk := false

	for {
		select {
		case _ = <-configMapReceivedAndProcessed:
			for _, event := range eventsIngress {
				c.eventChan <- event
			}
			for _, event := range eventsServices {
				c.eventChan <- event
			}
			for _, event := range eventsPods {
				c.eventChan <- event
			}
			eventsIngress = []SyncDataEvent{}
			eventsServices = []SyncDataEvent{}
			eventsPods = []SyncDataEvent{}
			configMapOk = true
			time.Sleep(1 * time.Millisecond)
		case item := <-cfgChan:
			c.eventChan <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: item.Namespace, Data: item}
		case item := <-nsChan:
			event := SyncDataEvent{SyncType: NAMESPACE, Namespace: item.Name, Data: item}
			c.eventChan <- event
		case item := <-podChan:
			event := SyncDataEvent{SyncType: POD, Namespace: item.Namespace, Data: item}
			if configMapOk {
				c.eventChan <- event
			} else {
				eventsPods = append(eventsPods, event)
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
		case <-time.After(time.Duration(syncEveryNSeconds) * time.Second):
			//TODO syncEveryNSeconds sec is hardcoded, change that (annotation?)
			//do sync of data every syncEveryNSeconds sec
			if configMapOk && len(eventsIngress) == 0 && len(eventsServices) == 0 && len(eventsPods) == 0 {
				c.eventChan <- SyncDataEvent{SyncType: COMMAND}
			}
		}
	}
}

//SyncData gets all kubernetes changes, aggregates them and apply to HAProxy.
//All the changes must come through this function
func (c *HAProxyController) SyncData(jobChan <-chan SyncDataEvent, chConfigMapReceivedAndProcessed chan bool) {
	hadChanges := false
	needsReload := false
	c.cfg.Init(c.osArgs, c.NativeAPI)
	for job := range jobChan {
		ns := c.cfg.GetNamespace(job.Namespace)
		change := false
		reload := false
		switch job.SyncType {
		case COMMAND:
			if hadChanges {
				if err := c.updateHAProxy(needsReload); err != nil {
					log.Println(err)
				}
				hadChanges = false
				needsReload = false
				continue
			}
		case NAMESPACE:
			change, reload = c.eventNamespace(ns, job.Data.(*Namespace))
		case INGRESS:
			change, reload = c.eventIngress(ns, job.Data.(*Ingress))
		case SERVICE:
			change, reload = c.eventService(ns, job.Data.(*Service))
		case POD:
			change, reload = c.eventPod(ns, job.Data.(*Pod))
		case CONFIGMAP:
			change, reload = c.eventConfigMap(ns, job.Data.(*ConfigMap), chConfigMapReceivedAndProcessed)
		case SECRET:
			change, reload = c.eventSecret(ns, job.Data.(*Secret))
		}
		hadChanges = hadChanges || change
		needsReload = needsReload || reload
	}
}
