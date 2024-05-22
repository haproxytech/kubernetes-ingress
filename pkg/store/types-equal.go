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
	"bytes"

	"github.com/haproxytech/client-native/v5/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func (a *ServicePort) Equal(b *ServicePort) bool {
	if a.Name != b.Name || a.Protocol != b.Protocol || a.Port != b.Port {
		return false
	}
	return true
}

// Equal compares two services, ignores statuses and old values
func (a *Service) Equal(b *Service) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if len(a.Annotations) != len(b.Annotations) {
		return false
	}
	for name, value1 := range a.Annotations {
		value2 := b.Annotations[name]
		if value1 != value2 {
			return false
		}
	}
	if len(a.Ports) != len(b.Ports) {
		return false
	}
	for index, p1 := range a.Ports {
		p2 := b.Ports[index]
		if p1.Name != p2.Name || p1.Protocol != p2.Protocol || p1.Port != p2.Port {
			return false
		}
	}
	return true
}

// Equal compares two config maps, ignores statuses and old values
func (a *ConfigMap) Equal(b *ConfigMap) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if len(a.Annotations) != len(b.Annotations) {
		return false
	}
	for name, value1 := range a.Annotations {
		value2 := b.Annotations[name]
		if value1 != value2 {
			return false
		}
	}
	return true
}

// Equal compares two secrets, ignores statuses and old values
func (a *Secret) Equal(b *Secret) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if len(a.Data) != len(b.Data) {
		return false
	}
	for key, value := range a.Data {
		value2, ok := b.Data[key]
		if !ok {
			return false
		}
		if !bytes.Equal(value, value2) {
			return false
		}
	}
	return true
}

// Equal checks if two services have same endpoints
func (a *Endpoints) Equal(b *Endpoints) bool {
	if a == nil || b == nil {
		return false
	}
	if a.SliceName != b.SliceName {
		return false
	}
	if a.Namespace != b.Namespace {
		return false
	}
	if a.Service != b.Service {
		return false
	}
	if len(a.Ports) != len(b.Ports) {
		return false
	}
	for portName, aPortValue := range a.Ports {
		bPortValue, ok := b.Ports[portName]
		if !ok || !aPortValue.Equal(bPortValue) {
			return false
		}
	}
	return true
}

// Equal checks if old PortEndpoints equals to a new PortEndpoints.
func (a *PortEndpoints) Equal(b *PortEndpoints) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Port != b.Port {
		return false
	}
	if len(a.Addresses) != len(b.Addresses) {
		return false
	}
	for addr := range a.Addresses {
		if _, ok := b.Addresses[addr]; !ok {
			return false
		}
	}
	return true
}

func (a *Service) EqualWithAddresses(b *Service) bool {
	return utils.EqualSliceStringsWithoutOrder(a.Addresses, b.Addresses)
}

func (gwc *GatewayClass) Equal(other *GatewayClass) bool {
	return gwc == nil && other == nil || (NoNilPointer(gwc, other) &&
		gwc.Name == other.Name &&
		gwc.ControllerName == other.ControllerName &&
		utils.EqualPointers(gwc.Description, other.Description))
}

func (gw *Gateway) Equal(other *Gateway) bool {
	return gw == nil && other == nil || (NoNilPointer(gw, other) &&
		gw.Name == other.Name &&
		gw.Namespace == other.Namespace &&
		gw.GatewayClassName == other.GatewayClassName &&
		Listeners(gw.Listeners).Equal(other.Listeners))
}

type Listeners []Listener

func (listeners Listeners) Equal(other Listeners) bool {
	if len(listeners) != len(other) {
		return false
	}
	mapListeners := map[string]Listener{}
	mapOtherListeners := map[string]Listener{}
	for _, listener := range listeners {
		mapListeners[listener.Name] = listener
	}

	for _, otherListener := range other {
		mapOtherListeners[otherListener.Name] = otherListener
	}

	for name, listener := range mapListeners {
		otherListener := mapOtherListeners[name]
		listenerCopy := listener
		if !otherListener.Equal(&listenerCopy) {
			return false
		}
	}

	for name, otherListener := range mapOtherListeners {
		listener := mapListeners[name]
		if !otherListener.Equal(&listener) {
			return false
		}
	}

	return true
}

func (listener *Listener) Equal(other *Listener) bool {
	return listener == nil && other == nil || (NoNilPointer(listener, other) &&
		listener.Name == other.Name &&
		listener.Port == other.Port &&
		listener.Protocol == other.Protocol &&
		utils.EqualPointers(listener.Hostname, other.Hostname) &&
		listener.AllowedRoutes.Equal(other.AllowedRoutes))
}

func (ar *AllowedRoutes) Equal(other *AllowedRoutes) bool {
	return ar == nil && other == nil ||
		(NoNilPointer(ar, other) && ar.Namespaces.Equal(other.Namespaces) && RouteGroupKinds(ar.Kinds).Equal(other.Kinds))
}

func (rn *RouteNamespaces) Equal(other *RouteNamespaces) bool {
	return rn == nil && other == nil || (NoNilPointer(rn, other) && utils.EqualPointers(rn.From, other.From) && rn.Selector.Equal(other.Selector))
}

func (backref *BackendRef) Equal(other *BackendRef) bool {
	return backref == nil && other == nil ||
		(NoNilPointer(backref, other) && backref.Name == other.Name && backref.Namespace == other.Namespace &&
			utils.EqualPointers(backref.Port, other.Port) && utils.EqualPointers(backref.Weight, other.Weight))
}

func (tcp *TCPRoute) Equal(other *TCPRoute) bool {
	return tcp == nil && other == nil || (NoNilPointer(tcp, other) &&
		tcp.Name == other.Name && tcp.Namespace == other.Namespace &&
		BackendRefs(tcp.BackendRefs).Equal(other.BackendRefs)) &&
		ParentRefs(tcp.ParentRefs).Equal(other.ParentRefs)
}

type BackendRefs []BackendRef

func (refs BackendRefs) Equal(other BackendRefs) bool {
	if len(refs) != len(other) {
		return false
	}

	mapBackendRefs := map[string]BackendRef{}
	mapOtherBackendRefs := map[string]BackendRef{}
	for _, backendRef := range refs {
		ns := "empty"
		if backendRef.Namespace != nil {
			ns = *backendRef.Namespace
		}
		mapBackendRefs[ns+"/"+backendRef.Name] = backendRef
	}
	for _, otherBackendRef := range other {
		ns := "empty"
		if otherBackendRef.Namespace != nil {
			ns = *otherBackendRef.Namespace
		}
		mapOtherBackendRefs[ns+"/"+otherBackendRef.Name] = otherBackendRef
	}

	for name, backendRef := range mapBackendRefs {
		backendRefCopy := backendRef
		otherBackendRef := mapOtherBackendRefs[name]
		if !otherBackendRef.Equal(&backendRefCopy) {
			return false
		}
	}

	for name, otherBackendRef := range mapOtherBackendRefs {
		backendRef := mapBackendRefs[name]
		if !otherBackendRef.Equal(&backendRef) {
			return false
		}
	}

	return true
}

func (labelSelector *LabelSelector) Equal(other *LabelSelector) bool {
	return labelSelector == nil && other == nil || (NoNilPointer(labelSelector, other) && utils.EqualMap(labelSelector.MatchLabels, other.MatchLabels) && utils.EqualSlice(labelSelector.MatchExpressions, other.MatchExpressions))
}

func (lsr LabelSelectorRequirement) Equal(other LabelSelectorRequirement, opt ...models.Options) bool {
	return lsr.Key == other.Key && lsr.Operator == other.Operator && utils.EqualSliceComparable(lsr.Values, other.Values)
}

func (ns *Namespace) Equal(other *Namespace) bool {
	return ns == nil && other == nil || (NoNilPointer(ns, other) && ns.Name == other.Name && utils.EqualMap(ns.Labels, other.Labels))
}

func (refto ReferenceGrantTo) Equal(other ReferenceGrantTo, opt ...models.Options) bool {
	return refto.Group == other.Group && refto.Kind == other.Kind && utils.EqualPointers(refto.Name, other.Name)
}

func (rf *ReferenceGrant) Equal(other *ReferenceGrant) bool {
	return rf == nil && other == nil ||
		(NoNilPointer(rf, other) && rf.Namespace == other.Namespace && rf.Name == other.Name && utils.EqualSliceComparable(rf.From, other.From) && utils.EqualSlice(rf.To, other.To))
}

type ParentRefs []ParentRef

func (refs ParentRefs) Equal(other ParentRefs) bool {
	if len(refs) != len(other) {
		return false
	}

	mapParentRefs := map[string]ParentRef{}
	mapOtherParentRefs := map[string]ParentRef{}
	for _, parentRef := range refs {
		ns := "empty"
		if parentRef.Namespace != nil {
			ns = *parentRef.Namespace
		}
		mapParentRefs[ns+"/"+parentRef.Name] = parentRef
	}
	for _, otherParentRef := range other {
		ns := "empty"
		if otherParentRef.Namespace != nil {
			ns = *otherParentRef.Namespace
		}
		mapOtherParentRefs[ns+"/"+otherParentRef.Name] = otherParentRef
	}

	for name, ParentRef := range mapParentRefs {
		otherBackendRef := mapOtherParentRefs[name]
		if !otherBackendRef.Equal(ParentRef) {
			return false
		}
	}

	for name, otherParentRef := range mapOtherParentRefs {
		parentRef := mapParentRefs[name]
		if !otherParentRef.Equal(parentRef) {
			return false
		}
	}

	return true
}

func (pr ParentRef) Equal(other ParentRef) bool {
	return pr.Name == other.Name && utils.EqualPointers(pr.Namespace, other.Namespace) && utils.EqualPointers(pr.Port, other.Port) && utils.EqualPointers(pr.SectionName, other.SectionName)
}

type RouteGroupKinds []RouteGroupKind

func (rgks RouteGroupKinds) Equal(other RouteGroupKinds) bool {
	if len(rgks) != len(other) {
		return false
	}

	mapRGK := map[string]struct{}{}
	mapOtherRGK := map[string]struct{}{}
	for _, rgk := range rgks {
		group := ""
		if rgk.Group != nil {
			group = *rgk.Group
		}

		mapRGK[group+"/"+rgk.Kind] = struct{}{}
	}
	for _, otherRgk := range other {
		group := ""
		if otherRgk.Group != nil {
			group = *otherRgk.Group
		}

		mapOtherRGK[group+"/"+otherRgk.Kind] = struct{}{}
	}

	for rgk := range mapRGK {
		_, found := mapOtherRGK[rgk]
		if !found {
			return false
		}
	}

	for otherRgk := range mapOtherRGK {
		_, found := mapRGK[otherRgk]
		if !found {
			return false
		}
	}

	return true
}

func NoNilPointer[P any](pointers ...*P) bool {
	for _, pointer := range pointers {
		if pointer == nil {
			return false
		}
	}
	return true
}

func (a *IngressClass) Equal(b *IngressClass) bool {
	if a == nil || b == nil {
		return false
	}

	return a.Controller == b.Controller && a.Name == b.Name
}
