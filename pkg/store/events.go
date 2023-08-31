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

import (
	"strings"

	"github.com/go-test/deep"
	corev1alpha2 "github.com/haproxytech/kubernetes-ingress/crs/api/core/v1alpha2"
)

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
	if data.Status == DELETED {
		delete(k.IngressClasses, data.Name)
	} else {
		k.IngressClasses[data.Name] = data
	}
	return true
}

func (k *K8s) EventIngress(ns *Namespace, data *Ingress) (updateRequired bool) {
	updateRequired = true

	if data.Status == DELETED {
		delete(ns.Ingresses, data.Name)
	} else {
		if oldIngress, ok := ns.Ingresses[data.Name]; ok {
			updated := deep.Equal(data.IngressCore, oldIngress.IngressCore)
			if len(updated) == 0 || (len(updated) == 1 && strings.HasSuffix(updated[0], "<nil pointer> != store.ServicePort")) {
				updateRequired = false
				data.Status = EMPTY
			}
		}
		ns.Ingresses[data.Name] = data
	}
	return
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
	if _, ok := ns.Endpoints[data.Service]; !ok {
		ns.Endpoints[data.Service] = make(map[string]*Endpoints)
	}
	if endpoints, ok := ns.Endpoints[data.Service][data.SliceName]; ok {
		if data.Status != DELETED && endpoints.Equal(data) {
			return false
		}
	}
	logger.Tracef("Treating endpoints event %+v", *data)
	ns.Endpoints[data.Service][data.SliceName] = data

	endpoints := getEndpoints(ns.Endpoints[data.Service])
	logger.Tracef("service %s : endpoints list %+v", data.Service, endpoints)
	_, ok := ns.HAProxyRuntime[data.Service]
	if !ok {
		ns.HAProxyRuntime[data.Service] = make(map[string]*RuntimeBackend)
	}
	logger.Tracef("service %s : number of already existing backend(s) in this transaction for this endpoint: %d", data.Service, len(ns.HAProxyRuntime[data.Service]))
	for key, value := range ns.HAProxyRuntime[data.Service] {
		logger.Tracef("service %s : port name %s, backend %+v", data.Service, key, *value)
	}
	for portName, portEndpoints := range endpoints {
		newBackend := &RuntimeBackend{Endpoints: portEndpoints}
		backend, ok := ns.HAProxyRuntime[data.Service][portName]
		if ok {
			portUpdated := (newBackend.Endpoints.Port != backend.Endpoints.Port)
			newBackend.HAProxySrvs = backend.HAProxySrvs
			newBackend.Name = backend.Name
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
		if oldService.Status == ADDED {
			newService.Status = ADDED
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
		cm.Annotations = map[string]string{}
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

func (k *K8s) EventGlobalCR(namespace, name string, data *corev1alpha2.Global) bool {
	ns := k.GetNamespace(namespace)
	if data == nil {
		delete(ns.CRs.Global, name)
		delete(ns.CRs.LogTargets, name)
		return true
	}
	ns.CRs.Global[name] = data.Spec.Config
	ns.CRs.LogTargets[name] = data.Spec.LogTargets
	return true
}

func (k *K8s) EventDefaultsCR(namespace, name string, data *corev1alpha2.Defaults) bool {
	ns := k.GetNamespace(namespace)
	if data == nil {
		delete(ns.CRs.Defaults, name)
		return true
	}
	ns.CRs.Defaults[name] = data.Spec.Config
	return true
}

func (k *K8s) EventBackendCR(namespace, name string, data *corev1alpha2.Backend) bool {
	ns := k.GetNamespace(namespace)
	if data == nil {
		delete(ns.CRs.Backends, name)
		return true
	}
	ns.CRs.Backends[name] = data.Spec.Config
	return true
}

func (k *K8s) EventPublishService(ns *Namespace, data *Service) (updateRequired bool) {
	updateRequired = false
	switch data.Status {
	case MODIFIED:
		newService := data
		oldService, ok := ns.Services[data.Name]
		if !ok {
			// intentionally do not add it. TODO see if our idea of only watching is ok
			logger.Warningf("Service '%s' not registered with controller !", data.Name)
		}
		if oldService.EqualWithAddresses(newService) {
			return
		}
		oldService.Addresses = newService.Addresses
		k.PublishServiceAddresses = oldService.Addresses
		// Extraction of ingresses from map to avoid concurrent modification.
		ingresses := []*Ingress{}
		for _, ns := range k.Namespaces {
			if !ns.Relevant {
				continue
			}
			for _, ingress := range k.Namespaces[ns.Name].Ingresses {
				ingresses = append(ingresses, ingress)
			}
		}
		go k.UpdateStatusFunc(ingresses, oldService.Addresses)
	case ADDED:
		if service, ok := ns.Services[data.Name]; ok {
			k.PublishServiceAddresses = data.Addresses
			service.Addresses = data.Addresses
			// Extraction of ingresses from map to avoid concurrent modification.
			ingresses := []*Ingress{}
			for _, ns := range k.Namespaces {
				if !ns.Relevant {
					continue
				}
				for _, ingress := range k.Namespaces[ns.Name].Ingresses {
					ingresses = append(ingresses, ingress)
				}
			}
			go k.UpdateStatusFunc(ingresses, service.Addresses)
			return
		}
		logger.Errorf("Publish service '%s/%s' not found", data.Namespace, data.Name)
	case DELETED:
		service, ok := ns.Services[data.Name]
		if ok {
			k.PublishServiceAddresses = nil
			service.Addresses = nil
			ingresses := []*Ingress{}
			for _, ns := range k.Namespaces {
				if !ns.Relevant {
					continue
				}
				for _, ingress := range k.Namespaces[ns.Name].Ingresses {
					ingresses = append(ingresses, ingress)
				}
			}
			go k.UpdateStatusFunc(ingresses, service.Addresses)
		} else {
			logger.Warningf("Publish service '%s/%s' not registered with controller, cannot delete !", data.Namespace, data.Name)
		}
	}
	return updateRequired
}
