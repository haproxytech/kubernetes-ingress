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

func (k *K8s) EventGatewayClass(data *GatewayClass) (updateRequired bool) {
	if data.ControllerName != k.GatewayControllerName {
		return updateRequired
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
			return updateRequired
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
	return updateRequired
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
			return updateRequired
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
	return updateRequired
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
			return updateRequired
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
	return updateRequired
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
			return updateRequired
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
	return updateRequired
}
