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

package store

func (k *K8s) EventNamespace(ns *Namespace, data *Namespace) (updateRequired bool) {
	updateRequired = false
	switch data.Status {
	case ADDED:
		_ = k.GetNamespace(data.Name)
	case DELETED:
		_, ok := k.Namespaces[data.Name]
		if ok {
			delete(k.Namespaces, data.Name)
			updateRequired = true
		} else {
			logger.Warningf("Namespace '%s' not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (k *K8s) EventIngressClass(data *IngressClass) (updateRequired bool) {
	switch data.Status {
	case MODIFIED:
		newIgClass := data
		oldIgClass, ok := k.IngressClasses[data.Name]
		if !ok {
			logger.Warningf("IngressClass '%s' not registered with controller !", data.Name)
			return false
		}
		if oldIgClass.Equal(oldIgClass) {
			return false
		}
		k.IngressClasses[data.Name] = newIgClass
		updateRequired = true
	case ADDED:
		if old, ok := k.IngressClasses[data.Name]; ok {
			if old.Status == DELETED {
				k.IngressClasses[data.Name].Status = ADDED
			}
			if !old.Equal(data) {
				data.Status = MODIFIED
				return k.EventIngressClass(data)
			}
			return false
		}
		k.IngressClasses[data.Name] = data
		updateRequired = true
	case DELETED:
		igClass, ok := k.IngressClasses[data.Name]
		if ok {
			igClass.Status = DELETED
			updateRequired = true
		} else {
			logger.Warningf("IngressClass '%s' not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (k *K8s) EventIngress(ns *Namespace, data *Ingress, controllerClass string) (updateRequired bool) {
	updateRequired = false
	switch data.Status {
	case MODIFIED:
		newIngress := data
		oldIngress, ok := ns.Ingresses[data.Name]
		if !ok {
			newIngress.Status = ADDED
			return k.EventIngress(ns, newIngress, controllerClass)
		}
		if oldIngress.Equal(data) {
			return false
		}
		for host, tls := range newIngress.TLS {
			_, ok := oldIngress.TLS[host]
			if !ok {
				tls.Status = ADDED
				continue
			}
		}
		for host, tls := range oldIngress.TLS {
			_, ok := newIngress.TLS[host]
			if !ok {
				newIngress.TLS[host] = &IngressTLS{
					Host:       host,
					SecretName: tls.SecretName,
					Status:     DELETED,
				}
				continue
			}
		}

		// so see what exactly has changed in there
		// DefaultBackend
		newDtBd := newIngress.DefaultBackend
		oldDtBd := oldIngress.DefaultBackend
		if newDtBd != nil && !oldDtBd.Equal(newDtBd) {
			newDtBd.Status = MODIFIED
		}
		// Rules
		for _, newRule := range newIngress.Rules {
			if oldRule, ok := oldIngress.Rules[newRule.Host]; ok {
				// so we need to compare if anything is different
				for _, newPath := range newRule.Paths {
					if oldPath, ok := oldRule.Paths[newPath.Path]; ok {
						// compare path for differences
						if newPath.SvcName != oldPath.SvcName ||
							newPath.SvcPortInt != oldPath.SvcPortInt ||
							newPath.SvcPortString != oldPath.SvcPortString {
							newPath.Status = MODIFIED
							newRule.Status = MODIFIED
						}
						// Sync internal data
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
		ns.Ingresses[data.Name] = newIngress
		updateRequired = true
	case ADDED:
		if old, ok := ns.Ingresses[data.Name]; ok {
			if old.Status == DELETED {
				ns.Ingresses[data.Name].Status = ADDED
			}
			data.Status = old.Status
			if !old.Equal(data) {
				data.Status = MODIFIED
				return k.EventIngress(ns, data, controllerClass)
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
			for _, tls := range ingress.TLS {
				tls.Status = DELETED
			}
			updateRequired = true
		} else {
			logger.Warningf("Ingress '%s' not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func getEndpoints(slices map[string]*Endpoints) (endpoints map[string]PortEndpoints) {
	endpoints = make(map[string]PortEndpoints)
	for _, slice := range slices {
		if slice.Status == DELETED {
			continue
		}
		for portName, portEndpoints := range slice.Ports {
			if _, ok := endpoints[portName]; !ok {
				endpoints[portName] = PortEndpoints{
					Port:      portEndpoints.Port,
					Addresses: make(map[string]struct{}),
				}
			}
			for address := range portEndpoints.Addresses {
				endpoints[portName].Addresses[address] = struct{}{}
			}
		}
	}
	return
}

func (k *K8s) EventEndpoints(ns *Namespace, data *Endpoints, syncHAproxySrvs func(backend *RuntimeBackend, portUpdated bool) error) (updateRequired bool) {
	newEndpoints := data
	oldEndpoints := ns.Endpoints[data.Service][data.SliceName]
	if oldEndpoints.Equal(newEndpoints) {
		return false
	}
	if _, ok := ns.Endpoints[data.Service]; !ok {
		ns.Endpoints[data.Service] = make(map[string]*Endpoints)
	}
	ns.Endpoints[data.Service][data.SliceName] = newEndpoints

	for portName, portEndpoints := range getEndpoints(ns.Endpoints[data.Service]) {
		newBackend := &RuntimeBackend{Endpoints: portEndpoints}
		runtime, ok := ns.HAProxyRuntime[data.Service]
		if !ok {
			runtime = make(map[string]*RuntimeBackend)
			ns.HAProxyRuntime[data.Service] = runtime
		}
		backend, ok := runtime[portName]
		if ok {
			portUpdated := (newBackend.Endpoints.Port != backend.Endpoints.Port)
			newBackend.HAProxySrvs = backend.HAProxySrvs
			newBackend.Name = backend.Name
			newBackend.Endpoints.Port = backend.Endpoints.Port
			logger.Warning(syncHAproxySrvs(newBackend, portUpdated))
		}
		ns.HAProxyRuntime[data.Service][portName] = newBackend
	}
	return true
}

func (k *K8s) EventService(ns *Namespace, data *Service) (updateRequired bool) {
	updateRequired = false
	switch data.Status {
	case MODIFIED:
		newService := data
		oldService, ok := ns.Services[data.Name]
		if !ok {
			// intentionally do not add it. TODO see if our idea of only watching is ok
			logger.Warningf("Service '%s' not registered with controller !", data.Name)
		}
		if oldService.Equal(newService) {
			return updateRequired
		}
		ns.Services[data.Name] = newService
		updateRequired = true
	case ADDED:
		if old, ok := ns.Services[data.Name]; ok {
			if old.Status == DELETED {
				ns.Services[data.Name].Status = ADDED
			}
			if !old.Equal(data) {
				data.Status = MODIFIED
				return k.EventService(ns, data)
			}
			return updateRequired
		}
		ns.Services[data.Name] = data
		updateRequired = true
	case DELETED:
		service, ok := ns.Services[data.Name]
		if ok {
			service.Status = DELETED
			updateRequired = true
		} else {
			logger.Warningf("Service '%s' not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (k *K8s) EventConfigMap(ns *Namespace, data *ConfigMap) (updateRequired bool) {
	var cm *ConfigMap
	switch {
	case k.ConfigMaps.Main.Namespace == ns.Name && k.ConfigMaps.Main.Name == data.Name:
		cm = k.ConfigMaps.Main
	case k.ConfigMaps.TCPServices.Namespace == ns.Name && k.ConfigMaps.TCPServices.Name == data.Name:
		cm = k.ConfigMaps.TCPServices
	case k.ConfigMaps.Errorfiles.Namespace == ns.Name && k.ConfigMaps.Errorfiles.Name == data.Name:
		cm = k.ConfigMaps.Errorfiles
	case k.ConfigMaps.PatternFiles.Namespace == ns.Name && k.ConfigMaps.PatternFiles.Name == data.Name:
		cm = k.ConfigMaps.PatternFiles
	default:
		return false
	}
	switch data.Status {
	case ADDED:
		if cm.Loaded && !cm.Equal(data) {
			data.Status = MODIFIED
			return k.EventConfigMap(ns, data)
		}
		*cm = *data
		cm.Loaded = true
		updateRequired = true
		logger.Debugf("configmap '%s/%s' processed", cm.Namespace, cm.Name)
	case MODIFIED:
		*cm = *data
		updateRequired = true
		logger.Infof("configmap '%s/%s' updated", cm.Namespace, cm.Name)
	case DELETED:
		cm.Loaded = false
		updateRequired = true
		logger.Debugf("configmap '%s/%s' deleted", cm.Namespace, cm.Name)
	}
	return updateRequired
}

func (k *K8s) EventSecret(ns *Namespace, data *Secret) (updateRequired bool) {
	updateRequired = false
	switch data.Status {
	case MODIFIED:
		newSecret := data
		oldSecret, ok := ns.Secret[data.Name]
		if !ok {
			// intentionally do not add it. TODO see if our idea of only watching is ok
			logger.Warningf("Secret '%s' not registered with controller !", data.Name)
			return updateRequired
		}
		if oldSecret.Equal(data) {
			return updateRequired
		}
		ns.Secret[data.Name] = newSecret
		updateRequired = true
	case ADDED:
		if old, ok := ns.Secret[data.Name]; ok {
			if old.Status == DELETED {
				ns.Secret[data.Name].Status = ADDED
			}
			if !old.Equal(data) {
				data.Status = MODIFIED
				return k.EventSecret(ns, data)
			}
			return updateRequired
		}
		ns.Secret[data.Name] = data
		updateRequired = true
	case DELETED:
		old, ok := ns.Secret[data.Name]
		if ok {
			updateRequired = true
			old.Status = DELETED
		} else {
			logger.Warningf("Secret '%s' not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (k *K8s) EventPod(podEvent PodEvent) (updateRequired bool) {
	if podEvent.Created {
		k.NbrHAProxyInst++
	} else {
		k.NbrHAProxyInst--
	}

	return true
}
