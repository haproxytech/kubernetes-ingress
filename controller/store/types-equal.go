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

import "bytes"

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
