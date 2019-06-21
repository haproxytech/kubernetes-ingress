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
	"fmt"
	"log"
	"strconv"
	"time"
)

func (c *HAProxyController) eventNamespace(ns *Namespace, data *Namespace) (updateRequired, needsReload bool) {
	updateRequired = false
	switch data.Status {
	case ADDED:
		_ = c.cfg.GetNamespace(data.Name)

	case DELETED:
		_, ok := c.cfg.Namespace[data.Name]
		if ok {
			delete(c.cfg.Namespace, data.Name)
			updateRequired = true
		} else {
			log.Println("Namespace not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired, updateRequired
}

func (c *HAProxyController) eventIngress(ns *Namespace, data *Ingress) (updateRequired, needsReload bool) {
	updateRequired = false
	switch data.Status {
	case MODIFIED:
		newIngress := data
		oldIngress, ok := ns.Ingresses[data.Name]
		if !ok {
			newIngress.Status = ADDED
			return c.eventIngress(ns, newIngress)
		}
		if oldIngress.Equal(data) {
			return false, false
		}
		newIngress.Annotations.SetStatus(oldIngress.Annotations)
		//so see what exactly has changed in there
		for _, newRule := range newIngress.Rules {
			if oldRule, ok := oldIngress.Rules[newRule.Host]; ok {
				//so we need to compare if anything is different
				for _, newPath := range newRule.Paths {
					if oldPath, ok := oldRule.Paths[newPath.Path]; ok {
						//compare path for differences
						if newPath.ServiceName != oldPath.ServiceName ||
							newPath.ServicePort != oldPath.ServicePort {
							newPath.Status = MODIFIED
							newRule.Status = MODIFIED
						}
					} else {
						newPath.Status = ADDED
						newRule.Status = ADDED
					}
				}
				for _, oldPath := range oldRule.Paths {
					if _, ok := newRule.Paths[oldPath.Path]; !ok {
						oldPath.Status = DELETED
						newRule.Paths[oldPath.Path] = oldPath
					}
				}
			} else {
				newRule.Status = ADDED
			}
		}
		for _, oldRule := range oldIngress.Rules {
			if _, ok := newIngress.Rules[oldRule.Host]; !ok {
				oldRule.Status = DELETED
				for _, path := range oldRule.Paths {
					path.Status = DELETED
				}
				newIngress.Rules[oldRule.Host] = oldRule
			}
		}
		for annName, ann := range newIngress.Annotations {
			annOLD, ok := oldIngress.Annotations[annName]
			if ok {
				if ann.Value != annOLD.Value {
					ann.OldValue = annOLD.Value
					ann.Status = MODIFIED
				}
			} else {
				ann.Status = ADDED
			}
		}
		for annName, ann := range oldIngress.Annotations {
			_, ok := oldIngress.Annotations[annName]
			if !ok {
				ann.Status = DELETED
			}
		}
		ns.Ingresses[data.Name] = newIngress
		//diffStr := cmp.Diff(oldIngress, newIngress)
		//log.Println("Ingress modified", data.Name, "\n", diffStr)
		updateRequired = true
	case ADDED:
		if old, ok := ns.Ingresses[data.Name]; ok {
			data.Status = old.Status
			if !old.Equal(data) {
				data.Status = MODIFIED
				return c.eventIngress(ns, data)
			}
			return updateRequired, updateRequired
		}
		ns.Ingresses[data.Name] = data

		for _, newRule := range data.Rules {
			for _, newPath := range newRule.Paths {
				newPath.Status = ADDED
			}
		}
		for _, ann := range data.Annotations {
			ann.Status = ADDED
		}

		//log.Println("Ingress added", data.Name)
		updateRequired = true
	case DELETED:
		ingress, ok := ns.Ingresses[data.Name]
		if ok {
			ingress.Status = DELETED
			for _, rule := range ingress.Rules {
				rule.Status = DELETED
				for _, path := range rule.Paths {
					path.Status = DELETED
				}
			}
			ingress.Annotations.SetStatusState(DELETED)
			//log.Println("Ingress deleted", data.Name)
			updateRequired = true
		} else {
			log.Println("Ingress not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired, updateRequired
}

func (c *HAProxyController) eventService(ns *Namespace, data *Service) (updateRequired, needsReload bool) {
	updateRequired = false
	switch data.Status {
	case MODIFIED:
		newService := data
		oldService, ok := ns.Services[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Service not registered with controller, cannot modify !", data.Name)
		}
		if oldService.Equal(newService) {
			return updateRequired, updateRequired
		}
		newService.Annotations.SetStatus(oldService.Annotations)
		ns.Services[data.Name] = newService
		updateRequired = true
	case ADDED:
		if old, ok := ns.Services[data.Name]; ok {
			if !old.Equal(data) {
				data.Status = MODIFIED
				return c.eventService(ns, data)
			}
			return updateRequired, updateRequired
		}
		ns.Services[data.Name] = data
		//log.Println("Service added", data.Name)
		updateRequired = true
	case DELETED:
		service, ok := ns.Services[data.Name]
		if ok {
			service.Status = DELETED
			service.Annotations.SetStatusState(DELETED)
			//log.Println("Service deleted", data.Name)
			updateRequired = true
		} else {
			log.Println("Service not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired, updateRequired
}

func (c *HAProxyController) reevaluatePod(data *Pod) {
	c.serverlessPodsLock.Lock()
	timer := c.serverlessPods[data.Name]
	c.serverlessPodsLock.Unlock()
	if timer == 0 {
		timer = 1
	} else if timer < 1000 {
		timer = 2 * timer
	} else {
		c.serverlessPodsLock.Lock()
		defer c.serverlessPodsLock.Unlock()
		delete(c.serverlessPods, data.Name)
		return
	}
	time.Sleep(time.Duration(timer) * time.Second)
	c.serverlessPodsLock.Lock()
	defer c.serverlessPodsLock.Unlock()
	c.serverlessPods[data.Name] = timer
	//log.Println(fmt.Sprintf("POD %s reevaluated after %d seconds", data.Name, timer))
	c.eventChan <- SyncDataEvent{SyncType: POD, Namespace: data.Namespace, Data: data}
}

func (c *HAProxyController) eventPod(ns *Namespace, data *Pod) (updateRequired, needsReload bool) {
	updateRequired = false
	needsReload = false
	runtimeClient := c.cfg.NativeAPI.Runtime
	switch data.Status {
	case MODIFIED:
		newPod := data
		var oldPod *Pod
		oldPod, ok := ns.Pods[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Pod not registered with controller, cannot modify !", data.Name)
			return updateRequired, needsReload
		}
		if oldPod.Equal(data) {
			return updateRequired, needsReload
		}
		newPod.HAProxyName = oldPod.HAProxyName
		newPod.Backends = oldPod.Backends
		if oldPod.Status == ADDED {
			newPod.Status = ADDED
		} else {
			//so, old is not just added, see diff and if only ip is different
			// issue socket command to change ip ad set it to ready
			if newPod.IP != oldPod.IP && len(oldPod.Backends) > 0 {
				for backendName := range newPod.Backends {
					err := runtimeClient.SetServerAddr(backendName, newPod.HAProxyName, newPod.IP, 0)
					if err != nil {
						log.Println(err)
						needsReload = true
					} else {
						log.Printf("POD modified through runtime: %s\n", data.Name)
					}
					err = runtimeClient.SetServerState(backendName, newPod.HAProxyName, "ready")
					if err != nil {
						log.Println(err)
						needsReload = true
					}
				}
			}
		}
		ns.Pods[data.Name] = newPod
		updateRequired = true
	case ADDED:
		if old, ok := ns.Pods[data.Name]; ok {
			data.HAProxyName = old.HAProxyName
			if old.Equal(data) {
				//so this is actually modified
				data.Status = MODIFIED
				return c.eventPod(ns, data)
			}
			return updateRequired, needsReload
		}
		//first see if we have spare place in servers
		//INFO if same pod used in multiple services, this will not work
		createNew := true
		var pods map[string]*Pod
		if services, err := ns.GetServicesForPod(data.Labels); err == nil {
			// we will see if we need to support behaviour where same pod is shared between services
			service := services[0]
			pods = ns.GetPodsForSelector(service.Selector)
			//now see if we have some free place where we can place pod
			for _, pod := range pods {
				if pod.Maintenance {
					createNew = false
					data.Maintenance = false
					if pod.Status == ADDED {
						data.Status = ADDED
					} else {
						data.Status = MODIFIED
					}
					data.HAProxyName = pod.HAProxyName
					data.Backends = pod.Backends
					ns.Pods[data.Name] = data
					delete(ns.Pods, pod.Name)
					updateRequired = true
					needsReload = false
					for backendName := range data.Backends {
						if data.IP != "" {
							err := runtimeClient.SetServerAddr(backendName, data.HAProxyName, data.IP, 0)
							if err != nil {
								log.Println(backendName, data.HAProxyName, data.IP, err)
								needsReload = true
							}
						}
						err := runtimeClient.SetServerState(backendName, data.HAProxyName, "ready")
						if err != nil {
							log.Println(backendName, data.HAProxyName, err)
							needsReload = true
						} else {
							log.Printf("POD added through runtime: %s\n", data.Name)
						}
					}
					break
				}
			}
			// in case we have delayed pod add
			c.serverlessPodsLock.Lock()
			defer c.serverlessPodsLock.Unlock()
			delete(c.serverlessPods, data.Name)
		} else {
			//no service or service data not yet available
			go c.reevaluatePod(data)
			createNew = false
		}
		if createNew {
			data.HAProxyName = fmt.Sprintf("SRV_%s", RandomString(5))
			for _, ok := ns.PodNames[data.HAProxyName]; ok; {
				data.HAProxyName = fmt.Sprintf("SRV_%s", RandomString(5))
			}
			ns.PodNames[data.HAProxyName] = true
			ns.Pods[data.Name] = data

			updateRequired = true
			needsReload = true

			annIncrement, _ := GetValueFromAnnotations("servers-increment", c.cfg.ConfigMap.Annotations)
			incrementSize := int64(128)
			if increment, err := strconv.ParseInt(annIncrement.Value, 10, 64); err == nil {
				incrementSize = increment
			}
			podsNumber := int64(len(pods) + 1)
			if podsNumber%incrementSize != 0 {
				for index := podsNumber % incrementSize; index < incrementSize; index++ {
					pod := &Pod{
						IP:          "127.0.0.1",
						Labels:      data.Labels.Clone(),
						Maintenance: true,
						Status:      ADDED,
					}
					pod.HAProxyName = fmt.Sprintf("SRV_%s", RandomString(5))
					for _, ok := ns.PodNames[pod.HAProxyName]; ok; {
						pod.HAProxyName = fmt.Sprintf("SRV_%s", RandomString(5))
					}
					pod.Name = pod.HAProxyName
					ns.PodNames[pod.HAProxyName] = true
					ns.Pods[pod.Name] = pod
				}
			}
		}
	case DELETED:
		oldPod, ok := ns.Pods[data.Name]
		if ok {
			if oldPod.Maintenance {
				//this occurres because we have a terminating signal (converted to delete)
				//and later we receive delete that is no longer relevant
				//log.Println("Pod already put to sleep !", data.Name)
				return updateRequired, needsReload
			}
			annIncrement, _ := GetValueFromAnnotations("servers-increment-max-disabled", c.cfg.ConfigMap.Annotations)
			maxDisabled := int64(8)
			if increment, err := strconv.ParseInt(annIncrement.Value, 10, 64); err == nil {
				maxDisabled = increment
			}
			var service *Service
			convertToMaintPod := true
			if services, err := ns.GetServicesForPod(data.Labels); err == nil {
				// we will see if we need to support behaviour where same pod is shared between services
				service = services[0]
				pods := ns.GetPodsForSelector(service.Selector)
				//first count number of disabled pods
				numDisabled := int64(0)
				for _, pod := range pods {
					if pod.Maintenance {
						numDisabled++
					}
				}
				if numDisabled >= maxDisabled {
					convertToMaintPod = false
					oldPod.Status = DELETED
					needsReload = true
				}
			}
			if convertToMaintPod {
				oldPod.IP = "127.0.0.1"
				oldPod.Status = MODIFIED //we replace it with disabled one
				oldPod.Maintenance = true
				for backendName := range oldPod.Backends {
					err := runtimeClient.SetServerState(backendName, oldPod.HAProxyName, "maint")
					if err != nil {
						log.Println(backendName, oldPod.HAProxyName, err)
					} else {
						log.Printf("POD disabled through runtime: %s\n", oldPod.Name)
					}
				}
			}
			updateRequired = true
		}
	}
	return updateRequired, needsReload
}

func (c *HAProxyController) eventConfigMap(ns *Namespace, data *ConfigMap, chConfigMapReceivedAndProcessed chan bool) (updateRequired, needsReload bool) {
	updateRequired = false
	if ns.Name != c.osArgs.ConfigMap.Namespace ||
		data.Name != c.osArgs.ConfigMap.Name {
		return updateRequired, needsReload
	}
	switch data.Status {
	case MODIFIED:
		different := data.Annotations.SetStatus(c.cfg.ConfigMap.Annotations)
		c.cfg.ConfigMap = data
		if !different {
			data.Status = EMPTY
		} else {
			updateRequired = true
		}
	case ADDED:
		if c.cfg.ConfigMap == nil {
			chConfigMapReceivedAndProcessed <- true
			c.cfg.ConfigMap = data
			updateRequired = true
			return updateRequired, updateRequired
		}
		if !c.cfg.ConfigMap.Equal(data) {
			data.Status = MODIFIED
			return c.eventConfigMap(ns, data, chConfigMapReceivedAndProcessed)
		}
	case DELETED:
		c.cfg.ConfigMap.Annotations.SetStatusState(DELETED)
		c.cfg.ConfigMap.Status = DELETED
	}
	return updateRequired, updateRequired
}
func (c *HAProxyController) eventSecret(ns *Namespace, data *Secret) (updateRequired, needsReload bool) {
	updateRequired = false
	switch data.Status {
	case MODIFIED:
		newSecret := data
		oldSecret, ok := ns.Secret[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Secret not registered with controller, cannot modify !", data.Name)
			return updateRequired, updateRequired
		}
		if oldSecret.Equal(data) {
			return updateRequired, updateRequired
		}
		ns.Secret[data.Name] = newSecret
		//result := cmp.Diff(oldSecret, newSecret)
		//log.Println("Secret modified", data.Name, "\n", result)
		updateRequired = true
	case ADDED:
		if old, ok := ns.Secret[data.Name]; ok {
			if !old.Equal(data) {
				data.Status = MODIFIED
				return c.eventSecret(ns, data)
			}
			return updateRequired, updateRequired
		}
		ns.Secret[data.Name] = data
		updateRequired = true
	case DELETED:
		_, ok := ns.Secret[data.Name]
		if ok {
			//log.Println("Secret set for deletion", data.Name)
			updateRequired = true
		} else {
			log.Println("Secret not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired, updateRequired
}
