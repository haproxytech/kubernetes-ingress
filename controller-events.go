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
)

func (c *HAProxyController) eventNamespace(ns *Namespace, data *Namespace) (updateRequired bool) {
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
	return updateRequired
}

func (c *HAProxyController) eventIngress(ns *Namespace, data *Ingress) (updateRequired bool) {
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
			return false
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
							newPath.ServicePortInt != oldPath.ServicePortInt ||
							newPath.ServicePortString != oldPath.ServicePortString {
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
			return updateRequired
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
	return updateRequired
}

func (c *HAProxyController) eventEndpoints(ns *Namespace, data *Endpoints) (updateRequired bool) {
	updateRequired = false
	switch data.Status {
	case MODIFIED:
		newEndpoints := data
		oldEndpoints, ok := ns.Endpoints[data.Service.Value]
		if !ok {
			log.Println("Endpoints not registered with controller !", data.Service)
			return updateRequired
		}
		if oldEndpoints.Equal(newEndpoints) {
			return updateRequired
		}
		data.BackendName = oldEndpoints.BackendName
		c.setModifiedStatusEndpoints(oldEndpoints, newEndpoints)
		updateRequired = updateRequired || c.processEndpointIPs(newEndpoints)
		ns.Endpoints[data.Service.Value] = newEndpoints
	case ADDED:
		if old, ok := ns.Endpoints[data.Service.Value]; ok {
			if !old.Equal(data) {
				data.Status = MODIFIED
				return c.eventEndpoints(ns, data)
			}
			return updateRequired
		}
		for _, ip := range *data.Addresses {
			ip.Status = ADDED
		}
		for _, port := range *data.Ports {
			port.Status = ADDED
		}
		ns.Endpoints[data.Service.Value] = data
		updateRequired = updateRequired || c.processEndpointIPs(data)
		//log.Println("Endpoints added", data.Service)
	case DELETED:
		oldData, ok := ns.Endpoints[data.Service.Value]
		if ok {
			oldData.Status = DELETED
			//log.Println("Endpoints deleted", data.Service)
			updateRequired = true
		} else {
			log.Println("Endpoints not registered with controller, cannot delete !", oldData.Service)
		}
	}
	return updateRequired
}

func (c *HAProxyController) setModifiedStatusEndpoints(oldObj, newObj *Endpoints) {
	if newObj.Namespace != oldObj.Namespace {
		newObj.Status = MODIFIED
	}
	if newObj.Service.Value != oldObj.Service.Value {
		newObj.Service.OldValue = oldObj.Service.Value
		newObj.Service.Status = MODIFIED
		newObj.Status = MODIFIED
	}
	for _, adrNew := range *newObj.Addresses {
		adrNew.Status = ADDED
	}
	for oldKey, adrOld := range *oldObj.Addresses {
		for _, adrNew := range *newObj.Addresses {
			if adrOld.IP == adrNew.IP {
				adrNew.HAProxyName = adrOld.HAProxyName
				adrNew.Status = adrOld.Status
				delete(*oldObj.Addresses, oldKey)
				break
			}
		}

	}
	for oldKey, adrOld := range *oldObj.Addresses {
		if !adrOld.Disabled {
			// it not disabled so it must be now, no longer exists
			adrOld.IP = "127.0.0.1"
			adrOld.Disabled = true
			adrOld.Status = MODIFIED
			(*newObj.Addresses)[oldKey] = adrOld
		} else {
			//try to find one that is added so we can switch them
			replaced := false
			for _, adrNew := range *newObj.Addresses {
				if adrNew.Status == ADDED {
					replaced = true
					adrNew.HAProxyName = adrOld.HAProxyName
					adrNew.Status = MODIFIED
					break
				}
			}
			if !replaced {
				(*newObj.Addresses)[oldKey] = adrOld
			}
		}

	}

	annIncrement, _ := GetValueFromAnnotations("servers-increment", c.cfg.ConfigMap.Annotations)
	incrementSize := int64(128)
	if increment, err := strconv.ParseInt(annIncrement.Value, 10, 64); err == nil {
		incrementSize = increment
	}
	numDisabled := int64(0)
	for _, adr := range *newObj.Addresses {
		if adr.Disabled {
			numDisabled++
		}
	}

	if numDisabled > incrementSize {
		alreadyDeleted := int64(0)
		for _, adr := range *newObj.Addresses {
			if adr.Status == DELETED {
				alreadyDeleted++
			}
		}
		division := numDisabled / incrementSize
		toDisable := division*incrementSize - alreadyDeleted
		if toDisable == 0 {
			return
		}
		for _, adr := range *newObj.Addresses {
			if adr.Disabled && toDisable > 0 {
				adr.IP = "127.0.0.1"
				adr.Status = DELETED
				toDisable--
			}
		}
	}
}

func (c *HAProxyController) processEndpointIPs(data *Endpoints) (updateRequired bool) {
	updateRequired = false
	annIncrement, _ := GetValueFromAnnotations("servers-increment", c.cfg.ConfigMap.Annotations)
	incrementSize := int64(128)
	if increment, err := strconv.ParseInt(annIncrement.Value, 10, 64); err == nil {
		incrementSize = increment
	}

	usedNames := map[string]struct{}{}
	for _, ip := range *data.Addresses {
		if ip.HAProxyName != "" {
			usedNames[ip.HAProxyName] = struct{}{}
		}
	}
	for _, ip := range *data.Addresses {
		switch ip.Status {
		case ADDED:
			//added on haproxy update
			ip.Status = ADDED
			ip.HAProxyName = fmt.Sprintf("SRV_%s", RandomString(5))
			for _, ok := usedNames[ip.HAProxyName]; ok; {
				ip.HAProxyName = fmt.Sprintf("SRV_%s", RandomString(5))
			}
			usedNames[ip.HAProxyName] = struct{}{}
			updateRequired = true
		case MODIFIED:
			if data.BackendName != "" {
				runtimeClient := c.cfg.NativeAPI.Runtime
				err := runtimeClient.SetServerAddr(data.BackendName, ip.HAProxyName, ip.IP, 0)
				if err != nil {
					log.Println(err)
					updateRequired = true
				}
				status := "ready"
				if ip.Disabled {
					status = "maint"
				}
				err = runtimeClient.SetServerState(data.BackendName, ip.HAProxyName, status)
				if err != nil {
					log.Println(err)
					updateRequired = true
				}
			} else {
				//this is ok since if exists, we edit current data
				ip.Status = ADDED
				updateRequired = true
			}
		case DELETED:
			//removed on haproxy update
			updateRequired = true
		}
	}

	//align new number of backend servers if necessary
	podsNumber := int64(len(*data.Addresses))
	if podsNumber%incrementSize == 0 {
		return updateRequired
	}
	toCreate := int(incrementSize - podsNumber%incrementSize)
	if toCreate == 0 {
		return updateRequired
	}
	for index := 0; index < toCreate; index++ {
		hAProxyName := fmt.Sprintf("SRV_%s", RandomString(5))
		for _, ok := usedNames[hAProxyName]; ok; {
			hAProxyName = fmt.Sprintf("SRV_%s", RandomString(5))
			usedNames[hAProxyName] = struct{}{}
		}

		(*data.Addresses)[hAProxyName] = &EndpointIP{
			IP:          "127.0.0.1",
			Name:        hAProxyName,
			HAProxyName: hAProxyName,
			Disabled:    true,
			Status:      ADDED,
		}
	}
	return updateRequired
}

func (c *HAProxyController) eventService(ns *Namespace, data *Service) (updateRequired bool) {
	updateRequired = false
	switch data.Status {
	case MODIFIED:
		newService := data
		oldService, ok := ns.Services[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Service not registered with controller !", data.Name)
		}
		if oldService.Equal(newService) {
			return updateRequired
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
			return updateRequired
		}
		ns.Services[data.Name] = data
		updateRequired = true
	case DELETED:
		service, ok := ns.Services[data.Name]
		if ok {
			service.Status = DELETED
			service.Annotations.SetStatusState(DELETED)
			updateRequired = true
		} else {
			log.Println("Service not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (c *HAProxyController) eventConfigMap(ns *Namespace, data *ConfigMap, chConfigMapReceivedAndProcessed chan bool) (updateRequired bool) {
	updateRequired = false
	if ns.Name != c.osArgs.ConfigMap.Namespace ||
		data.Name != c.osArgs.ConfigMap.Name {
		return updateRequired
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
			return updateRequired
		}
		if !c.cfg.ConfigMap.Equal(data) {
			data.Status = MODIFIED
			return c.eventConfigMap(ns, data, chConfigMapReceivedAndProcessed)
		}
	case DELETED:
		c.cfg.ConfigMap.Annotations.SetStatusState(DELETED)
		c.cfg.ConfigMap.Status = DELETED
	}
	return updateRequired
}
func (c *HAProxyController) eventSecret(ns *Namespace, data *Secret) (updateRequired bool) {
	updateRequired = false
	switch data.Status {
	case MODIFIED:
		newSecret := data
		oldSecret, ok := ns.Secret[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Secret not registered with controller !", data.Name)
			return updateRequired
		}
		if oldSecret.Equal(data) {
			return updateRequired
		}
		ns.Secret[data.Name] = newSecret
		updateRequired = true
	case ADDED:
		if old, ok := ns.Secret[data.Name]; ok {
			if !old.Equal(data) {
				data.Status = MODIFIED
				return c.eventSecret(ns, data)
			}
			return updateRequired
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
	return updateRequired
}
