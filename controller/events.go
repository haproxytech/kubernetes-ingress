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
	"fmt"
	"strconv"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
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
			c.Logger.Warningf("Namespace '%s' not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (c *HAProxyController) eventIngress(ns *Namespace, data *Ingress) (updateRequired bool) {
	ingressClass := ""
	updateRequired = false
	switch data.Status {
	case MODIFIED:
		newIngress := data
		oldIngress, ok := ns.Ingresses[data.Name]
		if !ok {
			newIngress.Status = ADDED
			return c.eventIngress(ns, newIngress)
		}
		if annIngressClass, ok := data.Annotations["ingress.class"]; ok {
			ingressClass = annIngressClass.Value
		}
		if ingressClass != c.cfg.IngressClass {
			newIngress.Status = DELETED
			return c.eventIngress(ns, newIngress)
		}
		if oldIngress.Equal(data) {
			return false
		}
		newIngress.Annotations.SetStatus(oldIngress.Annotations)
		for host, tls := range newIngress.TLS {
			old, ok := oldIngress.TLS[host]
			if !ok {
				tls.Status = ADDED
				continue
			}
			tls.SecretName.OldValue = old.SecretName.Value
		}
		for host, tls := range oldIngress.TLS {
			_, ok := newIngress.TLS[host]
			if !ok {
				newIngress.TLS[host] = &IngressTLS{
					Host: host,
					SecretName: StringW{
						Value: tls.SecretName.Value,
					},
					Status: DELETED,
				}
				continue
			}
		}

		//so see what exactly has changed in there
		//DefaultBackend
		newDtBd := newIngress.DefaultBackend
		oldDtBd := oldIngress.DefaultBackend
		if newDtBd != nil && !oldDtBd.Equal(newDtBd) {
			newDtBd.Status = MODIFIED
		}
		//Rules
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
						// Sync internal data
						newPath.IsTCPService = oldPath.IsTCPService
						newPath.IsSSLPassthrough = oldPath.IsSSLPassthrough
						newPath.IsDefaultBackend = oldPath.IsDefaultBackend
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
				for _, path := range newRule.Paths {
					path.Status = ADDED
				}
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
		// Annotations
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
		//c.Logger.Tracef("Ingress modified %s %s", data.Name, diffStr)
		updateRequired = true
	case ADDED:
		if annIngressClass, ok := data.Annotations["ingress.class"]; ok {
			ingressClass = annIngressClass.Value
		}
		if ingressClass != c.cfg.IngressClass {
			return false
		}
		if old, ok := ns.Ingresses[data.Name]; ok {
			data.Status = old.Status
			if !old.Equal(data) {
				data.Status = MODIFIED
				return c.eventIngress(ns, data)
			}
			return updateRequired
		}
		ns.Ingresses[data.Name] = data

		if data.DefaultBackend != nil {
			data.DefaultBackend.Status = ADDED
		}
		for _, newRule := range data.Rules {
			for _, newPath := range newRule.Paths {
				newPath.Status = ADDED
			}
		}
		for _, ann := range data.Annotations {
			ann.Status = ADDED
		}
		for _, tls := range data.TLS {
			tls.Status = ADDED
		}
		updateRequired = true
	case DELETED:
		ingress, ok := ns.Ingresses[data.Name]
		if ok {
			ingress.Status = DELETED
			if ingress.DefaultBackend != nil {
				ingress.DefaultBackend.Status = DELETED
			}
			for _, rule := range ingress.Rules {
				rule.Status = DELETED
				for _, path := range rule.Paths {
					path.Status = DELETED
				}
			}
			ingress.Annotations.SetStatusState(DELETED)
			updateRequired = true
		} else {
			c.Logger.Warningf("Ingress '%s' not registered with controller, cannot delete !", data.Name)
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
			c.Logger.Warningf("Endpoints '%s' not registered with controller !", data.Service)
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
	case DELETED:
		oldData, ok := ns.Endpoints[data.Service.Value]
		if ok {
			oldData.Status = DELETED
			updateRequired = true
		} else {
			c.Logger.Warningf("Endpoints '%s' not registered with controller, cannot delete !", oldData.Service)
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
			(*newObj.Addresses)[fmt.Sprintf("SRV_%s", utils.RandomString(5))] = adrOld
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
			ip.HAProxyName = fmt.Sprintf("SRV_%s", utils.RandomString(5))
			for _, ok := usedNames[ip.HAProxyName]; ok; {
				ip.HAProxyName = fmt.Sprintf("SRV_%s", utils.RandomString(5))
			}
			usedNames[ip.HAProxyName] = struct{}{}
			updateRequired = true
		case MODIFIED:
			if data.BackendName != "" {
				err := c.Client.SetServerAddr(data.BackendName, ip.HAProxyName, ip.IP, 0)
				if err != nil {
					c.Logger.Error(err)
					updateRequired = true
				}
				status := "ready"
				if ip.Disabled {
					status = "maint"
				}
				c.Logger.Debugf("server '%s/%s' changed status to %v", data.BackendName, ip.HAProxyName, status)
				err = c.Client.SetServerState(data.BackendName, ip.HAProxyName, status)
				if err != nil {
					c.Logger.Error(err)
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
		hAProxyName := fmt.Sprintf("SRV_%s", utils.RandomString(5))
		for _, ok := usedNames[hAProxyName]; ok; {
			hAProxyName = fmt.Sprintf("SRV_%s", utils.RandomString(5))
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
			c.Logger.Warningf("Service '%s' not registered with controller !", data.Name)
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
			c.Logger.Warningf("Service '%s' not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (c *HAProxyController) eventConfigMap(ns *Namespace, data *ConfigMap, chConfigMapReceivedAndProcessed chan bool) (updateRequired bool) {
	updateRequired = false
	//TODO refractor this so we remember all configmaps, since we now use more that one
	configmap := false
	configmapTCP := false
	configmapErrorfile := false

	if ns.Name == c.osArgs.ConfigMap.Namespace && data.Name == c.osArgs.ConfigMap.Name {
		configmap = true
	}
	if ns.Name == c.osArgs.ConfigMapTCPServices.Namespace && data.Name == c.osArgs.ConfigMapTCPServices.Name {
		configmapTCP = true
	}
	if ns.Name == c.osArgs.ConfigMapErrorfiles.Namespace && data.Name == c.osArgs.ConfigMapErrorfiles.Name {
		configmapErrorfile = true
	}

	if configmap {
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
	}

	if configmapTCP {
		switch data.Status {
		case MODIFIED:
			different := data.Annotations.SetStatus(c.cfg.ConfigMapTCPServices.Annotations)
			c.cfg.ConfigMapTCPServices = data
			if !different {
				data.Status = EMPTY
			} else {
				updateRequired = true
			}
		case ADDED:
			if c.cfg.ConfigMapTCPServices == nil {
				c.cfg.ConfigMapTCPServices = data
				updateRequired = true
				return updateRequired
			}
			if !c.cfg.ConfigMapTCPServices.Equal(data) {
				data.Status = MODIFIED
				return c.eventConfigMap(ns, data, chConfigMapReceivedAndProcessed)
			}
		case DELETED:
			c.cfg.ConfigMapTCPServices.Annotations.SetStatusState(DELETED)
			c.cfg.ConfigMapTCPServices.Status = DELETED
		}
	}

	if configmapErrorfile {
		switch data.Status {
		case MODIFIED:
			different := data.Annotations.SetStatus(c.cfg.ConfigMapErrorfile.Annotations)
			c.cfg.ConfigMapErrorfile = data
			if !different {
				data.Status = EMPTY
			} else {
				updateRequired = true
			}
		case ADDED:
			if c.cfg.ConfigMapErrorfile == nil {
				c.cfg.ConfigMapErrorfile = data
				updateRequired = true
				return updateRequired
			}
			if !c.cfg.ConfigMapErrorfile.Equal(data) {
				data.Status = MODIFIED
				return c.eventConfigMap(ns, data, chConfigMapReceivedAndProcessed)
			}
		case DELETED:
			c.cfg.ConfigMapErrorfile.Annotations.SetStatusState(DELETED)
			c.cfg.ConfigMapErrorfile.Status = DELETED
		}
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
			c.Logger.Warningf("Secret '%s' not registered with controller !", data.Name)
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
			updateRequired = true
		} else {
			c.Logger.Warningf("Secret '%s' not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}
