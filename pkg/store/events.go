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
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s/meta"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
)

func (k *K8s) EventNamespace(ns *Namespace, data *Namespace) (updateRequired bool) {
	updateRequired = false
	switch data.Status {
	case ADDED:
		nsStore := k.GetNamespace(data.Name)
		nsStore.Labels = utils.CopyMap(data.Labels)
		updateRequired = true
	case MODIFIED:
		nsStore := k.GetNamespace(data.Name)
		updateRequired = !utils.EqualMap(nsStore.Labels, data.Labels)
		if updateRequired {
			nsStore.Labels = utils.CopyMap(data.Labels)
		}
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
		updateRequired = true
	} else if !data.Equal(k.IngressClasses[data.Name]) {
		updateRequired = true
		k.IngressClasses[data.Name] = data
	}
	return updateRequired
}

func (k *K8s) EventIngress(ns *Namespace, data *Ingress, uid types.UID, resourceVersion string) (updateRequired bool) {
	updateRequired = true

	if data.Status == DELETED {
		delete(ns.Ingresses, data.Name)
		for _, rule := range data.Rules {
			for _, path := range rule.Paths {
				k.IngressesByService[path.SvcNamespace+"/"+path.SvcName].Remove(data)
			}
		}
		meta.GetMetaStore().ProcessedResourceVersion.Delete(data, uid)
	} else {
		if oldIngress, ok := ns.Ingresses[data.Name]; ok {
			updated := deep.Equal(data.IngressCore, oldIngress.IngressCore)
			if len(updated) == 0 || (len(updated) == 1 && strings.HasSuffix(updated[0], "<nil pointer> != store.ServicePort")) {
				updateRequired = false
				data.Status = EMPTY
			}
			for _, update := range updated {
				if strings.HasPrefix(update, "Class:") {
					data.ClassUpdated = true
					break
				}
			}
			if data.Annotations["ingress.class"] != oldIngress.Annotations["ingress.class"] {
				data.ClassUpdated = true
			}

			for _, rule := range oldIngress.Rules {
				for _, path := range rule.Paths {
					k.IngressesByService[path.SvcNamespace+"/"+path.SvcName].Remove(data)
				}
			}
		}
		ns.Ingresses[data.Name] = data
		meta.GetMetaStore().ProcessedResourceVersion.Set(data, uid, resourceVersion)

		for _, rule := range data.Rules {
			for _, path := range rule.Paths {
				key := path.SvcNamespace + "/" + path.SvcName
				ingresses := k.IngressesByService[key]

				if ingresses == nil {
					ingresses = utils.NewOrderedSet[string, *Ingress](func(i *Ingress) string { return i.Name },
						func(a, b *Ingress) bool {
							// We need to consider the case where two ingresses are created in the same second.
							// Otherwise there could be any order in two instances;
							if a.CreationTime.Equal(b.CreationTime) {
								return a.Namespace+a.Name < b.Namespace+b.Name
							}
							return a.CreationTime.After(b.CreationTime)
						})
					k.IngressesByService[key] = ingresses
				}
				ingresses.Add(data)
			}
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
	return endpoints
}

func (k *K8s) EventEndpoints(ns *Namespace, data *Endpoints, syncHAproxySrvs func(backend *RuntimeBackend, portUpdated bool) error) (updateRequired bool) {
	if _, ok := ns.Endpoints[data.Service]; !ok {
		ns.Endpoints[data.Service] = make(map[string]*Endpoints)
	}
	if endpoints, ok := ns.Endpoints[data.Service][data.SliceName]; ok {
		if data.Status != DELETED && endpoints.Equal(data) {
			if data != nil {
				logger.Tracef("[RUNTIME] [BACKEND] [SERVER] [No change] [EventEndpoints]. No change for %s %s %s", data.Status, data.Service, data.SliceName)
			}
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
	// Standalone
	_, ok = ns.HAProxyRuntimeStandalone[data.Service]
	if !ok {
		ns.HAProxyRuntimeStandalone[data.Service] = make(map[string]map[string]*RuntimeBackend)
	}
	for key, value := range ns.HAProxyRuntime[data.Service] {
		logger.Tracef("service %s : port name %s, backend %+v", data.Service, key, *value)
	}
	// Standalone
	for portName, backendsNames := range ns.HAProxyRuntimeStandalone[data.Service] {
		for backendName := range backendsNames {
			logger.Tracef("service %s : port name %s, backend %+v", data.Service, portName, backendName)
		}
	}
	if len(endpoints) == 0 {
		for _, runtimeBackend := range ns.HAProxyRuntime[data.Service] {
			runtimeBackend.Endpoints = PortEndpoints{}
			for _, haproxySrv := range runtimeBackend.HAProxySrvs {
				haproxySrv.Address = ""
				haproxySrv.Modified = true
			}
		}
	}
	for portName, portEndpoints := range endpoints {
		// Make a copy of addresses for potential standalone runtime backend
		// as these addresses are consumed/removed in the process
		backendAddresses := utils.CopyMap(portEndpoints.Addresses)
		newBackend := &RuntimeBackend{Endpoints: portEndpoints}
		backend, ok := ns.HAProxyRuntime[data.Service][portName]
		// Make a copy of haproxy server list for potential standalone runtime backend
		// as this servere list is modified in the process
		var backendHAProxySrvs []*HAProxySrv
		if ok {
			backendHAProxySrvs = utils.CopySliceFunc(backend.HAProxySrvs, utils.CopyPointer)
			portUpdated := (newBackend.Endpoints.Port != backend.Endpoints.Port)
			newBackend.HAProxySrvs = backend.HAProxySrvs
			newBackend.Name = backend.Name
			logger.Warning(syncHAproxySrvs(newBackend, portUpdated))
		}
		ns.HAProxyRuntime[data.Service][portName] = newBackend

		// Reprocuce the same steps ar regular runtime backend for each standalone runtime backend
		// referring to the same port and service
		standaloneNewBackend := &RuntimeBackend{Endpoints: portEndpoints}
		for standaloneBackendName, standaloneRuntimeBackend := range ns.HAProxyRuntimeStandalone[data.Service][portName] {
			// Make own copy of regular runtime backend portEndpoint addresses
			standaloneNewBackend.Endpoints.Addresses = utils.CopyMap(backendAddresses)
			// Make own copy of regular runtime backend portEndpoint servers list
			standaloneNewBackend.HAProxySrvs = utils.CopySliceFunc(backendHAProxySrvs, utils.CopyPointer)
			standaloneNewBackend.Name = standaloneRuntimeBackend.Name
			standalonePortUpdated := (standaloneNewBackend.Endpoints.Port != standaloneRuntimeBackend.Endpoints.Port)
			logger.Warning(syncHAproxySrvs(standaloneNewBackend, standalonePortUpdated))
			ns.HAProxyRuntimeStandalone[data.Service][portName][standaloneBackendName] = standaloneNewBackend
			standaloneNewBackend = &RuntimeBackend{Endpoints: portEndpoints}
		}
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
			// It can happen (resync) that we receive an UPDATE on a item that is not yet registered
			// We should treat it as a CREATE.
			logger.Warningf("Service '%s' not registered with controller !", data.Name)
			data.Status = ADDED
			return k.EventService(ns, data)
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
		if !cm.Empty() {
			updateRequired = true
		}
		logger.Debugf("configmap '%s/%s' processed", cm.Namespace, cm.Name)
	case MODIFIED:
		if cm.Equal(data) {
			return false
		}
		*cm = *data
		updateRequired = true
		logger.Infof("configmap '%s/%s' updated", cm.Namespace, cm.Name)
	case DELETED:
		cm.Loaded = false
		cm.Annotations = map[string]string{}
		if !cm.Empty() {
			updateRequired = true
		}
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
			// It can happen (resync) that we receive an UPDATE on a item that is not yet registered
			// We should treat it as a CREATE.
			logger.Warningf("Secret '%s' not registered with controller !", data.Name)
			data.Status = ADDED
			return k.EventSecret(ns, data)
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
	switch podEvent.Status {
	case ADDED, MODIFIED:
		if _, ok := k.HaProxyPods[podEvent.Name]; ok {
			return false
		}
		k.HaProxyPods[podEvent.Name] = struct{}{}
	case DELETED:
		if _, ok := k.HaProxyPods[podEvent.Name]; !ok {
			return false
		}
		delete(k.HaProxyPods, podEvent.Name)
	}

	return true
}

func (k *K8s) EventPublishService(ns *Namespace, data *Service) (updateRequired bool) {
	updateRequired = false
	switch data.Status {
	case MODIFIED:
		newService := data
		oldService, ok := ns.Services[data.Name]
		if !ok {
			// It can happen (resync) that we receive an UPDATE on a item that is not yet registered
			// We should treat it as a CREATE.
			logger.Warningf("Service '%s' not registered with controller !", data.Name)
			data.Status = ADDED
			return k.EventPublishService(ns, data)
		}
		if oldService.EqualWithAddresses(newService) {
			return updateRequired
		}
		oldService.Addresses = newService.Addresses
		k.PublishServiceAddresses = newService.Addresses
		k.UpdateAllIngresses = true
		updateRequired = true
	case ADDED:
		if service, ok := ns.Services[data.Name]; ok {
			k.PublishServiceAddresses = data.Addresses
			service.Addresses = data.Addresses
			k.UpdateAllIngresses = true
			updateRequired = true
			return updateRequired
		}
		logger.Errorf("Publish service '%s/%s' not found", data.Namespace, data.Name)
	case DELETED:
		service, ok := ns.Services[data.Name]
		if ok {
			k.PublishServiceAddresses = nil
			service.Addresses = nil
			k.UpdateAllIngresses = true
			updateRequired = true
		} else {
			logger.Warningf("Publish service '%s/%s' not registered with controller, cannot delete !", data.Namespace, data.Name)
		}
	}
	return updateRequired
}
