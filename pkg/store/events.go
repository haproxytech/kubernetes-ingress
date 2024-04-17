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
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
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
		updateRequired = !EqualMap(nsStore.Labels, data.Labels)
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
	return
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
			for _, update := range updated {
				if strings.HasPrefix(update, "Class:") {
					data.ClassUpdated = true
					break
				}
			}
			if data.Annotations["ingress.class"] != oldIngress.Annotations["ingress.class"] {
				data.ClassUpdated = true
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
		updateRequired = true
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
			// It can happen (resync) that we receive an UPDATE on a item that is not yet registered
			// We should treat it as a CREATE.
			logger.Warningf("Service '%s' not registered with controller !", data.Name)
			data.Status = ADDED
			return k.EventPublishService(ns, data)
		}
		if oldService.EqualWithAddresses(newService) {
			return
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
			return
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

func (k *K8s) EventGatewayClass(data *GatewayClass) (updateRequired bool) {
	if data.ControllerName != k.GatewayControllerName {
		return
	}
	switch data.Status {
	case ADDED:

		if previous := k.GatewayClasses[data.Name]; previous != nil {
			logger.Warningf("Replacing existing gatewayclass %s", data.Name)
		}
		k.GatewayClasses[data.Name] = data
		updateRequired = true
	case DELETED:
		if previous := k.GatewayClasses[data.Name]; previous == nil {
			logger.Warningf("Trying to delete unexisting gatewayclass %s", data.Name)
			return
		}
		delete(k.GatewayClasses, data.Name)
		updateRequired = true
	case MODIFIED:
		newGatewayClass := data
		oldGatewayClass, ok := k.GatewayClasses[data.Name]
		if !ok {
			// It can happen (resync) that we receive an UPDATE on a item that is not yet registered
			// We should treat it as a CREATE.
			logger.Warningf("Modification of unexisting gatewayclass %s", data.Name)
			data.Status = ADDED
			return k.EventGatewayClass(data)
		}
		if ok && oldGatewayClass.Generation == newGatewayClass.Generation ||
			newGatewayClass.Equal(oldGatewayClass) {
			return updateRequired
		}
		k.GatewayClasses[data.Name] = newGatewayClass
		updateRequired = true
	}
	return
}

func (k *K8s) EventGateway(ns *Namespace, data *Gateway) (updateRequired bool) {
	switch data.Status {
	case ADDED:
		if previous := ns.Gateways[data.Name]; previous != nil {
			logger.Warningf("Replacing existing gateway %s", data.Name)
		}
		ns.Gateways[data.Name] = data
		updateRequired = true
	case DELETED:
		if previous := ns.Gateways[data.Name]; previous == nil {
			logger.Warningf("Trying to delete unexisting gateway %s", data.Name)
			return
		}
		ns.Gateways[data.Name] = data
		updateRequired = true
	case MODIFIED:
		newGateway := data
		oldGateway, ok := ns.Gateways[data.Name]
		if !ok {
			// It can happen (resync) that we receive an UPDATE on a item that is not yet registered
			// We should treat it as a CREATE.
			logger.Warningf("Modification of unexisting gateway %s", data.Name)
			data.Status = ADDED
			return k.EventGateway(ns, data)
		}
		if ok && newGateway.Generation == oldGateway.Generation ||
			newGateway.Equal(oldGateway) {
			return updateRequired
		}
		ns.Gateways[data.Name] = newGateway
		updateRequired = true
	}
	return
}

func (k *K8s) EventTCPRoute(ns *Namespace, data *TCPRoute) (updateRequired bool) {
	switch data.Status {
	case ADDED:
		if previous := ns.TCPRoutes[data.Name]; previous != nil {
			logger.Warningf("Replacing existing tcproute %s", data.Name)
		}
		ns.TCPRoutes[data.Name] = data
		updateRequired = true
	case DELETED:
		if previous := ns.TCPRoutes[data.Name]; previous == nil {
			logger.Warningf("Trying to delete unexisting tcproute %s", data.Name)
			return
		}
		// We can't remove directly because we need the listener attached to this route to be updated.
		ns.TCPRoutes[data.Name] = data
		updateRequired = true
	case MODIFIED:
		newTCPRoute := data
		oldTCPRoute, ok := ns.TCPRoutes[data.Name]
		if !ok {
			// It can happen (resync) that we receive an UPDATE on a item that is not yet registered
			// We should treat it as a CREATE.
			logger.Warningf("Modification of unexisting tcproute %s", data.Name)
			data.Status = ADDED
			return k.EventTCPRoute(ns, data)
		}
		if ok && newTCPRoute.Generation == oldTCPRoute.Generation ||
			newTCPRoute.Equal(oldTCPRoute) {
			return false
		}
		ns.TCPRoutes[data.Name] = newTCPRoute
		updateRequired = true
	}
	return
}

func (k *K8s) EventReferenceGrant(ns *Namespace, data *ReferenceGrant) (updateRequired bool) {
	switch data.Status {
	case ADDED:
		if previous := ns.ReferenceGrants[data.Name]; previous != nil {
			logger.Warningf("Replacing existing referencegrant %s", data.Name)
		}
		ns.ReferenceGrants[data.Name] = data
		updateRequired = true
	case DELETED:
		if previous := ns.ReferenceGrants[data.Name]; previous == nil {
			logger.Warningf("Trying to delete unexisting refrencegrant %s", data.Name)
			return
		}
		delete(ns.ReferenceGrants, data.Name)
		updateRequired = true
	case MODIFIED:
		newReferenceGrant := data
		oldReferenceGrant, ok := ns.ReferenceGrants[data.Name]
		if !ok {
			// It can happen (resync) that we receive an UPDATE on a item that is not yet registered
			// We should treat it as a CREATE.
			logger.Warningf("Modification of unexisting referencegrant %s", data.Name)
			data.Status = ADDED
			return k.EventReferenceGrant(ns, data)
		}
		if ok && newReferenceGrant.Generation == oldReferenceGrant.Generation ||
			newReferenceGrant.Equal(oldReferenceGrant) {
			return updateRequired
		}
		ns.ReferenceGrants[data.Name] = newReferenceGrant
		updateRequired = true
	}
	return
}
