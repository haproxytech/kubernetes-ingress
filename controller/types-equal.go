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

import "bytes"

func (a *ServicePort) Equal(b *ServicePort) bool {
	if a.Name != b.Name || a.Protocol != b.Protocol || a.Port != b.Port {
		return false
	}
	return true
}

//Equal checks if Ingress Paths are equal
func (a *IngressPath) Equal(b *IngressPath) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Path != b.Path {
		return false
	}
	if a.ServiceName != b.ServiceName {
		return false
	}
	if a.ServicePortInt != b.ServicePortInt {
		return false
	}
	if a.ServicePortString != b.ServicePortString {
		return false
	}
	return true
}

//Equal checks if Ingress Rules are equal
func (a *IngressRule) Equal(b *IngressRule) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Host != b.Host {
		return false
	}
	if len(a.Paths) != len(b.Paths) {
		return false
	}
	for key, value := range a.Paths {
		value2, ok := b.Paths[key]
		if !ok || !value.Equal(value2) {
			return false
		}

	}
	return true
}

//Equal checks if Ingress secrets are equal
func (a *IngressTLS) Equal(b *IngressTLS) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Host != b.Host {
		return false
	}
	if a.SecretName != b.SecretName {
		return false
	}
	return true
}

//Equal compares two Ingresses, ignores
func (a *Ingress) Equal(b *Ingress) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if len(a.Rules) != len(b.Rules) {
		return false
	}
	for k, v := range a.Rules {
		value, ok := b.Rules[k]
		if !ok || !v.Equal(value) {
			return false
		}
	}
	if len(a.TLS) != len(b.TLS) {
		return false
	}
	for k, v := range a.TLS {
		value, ok := b.TLS[k]
		if !ok || !v.Equal(value) {
			return false
		}
	}
	if a.DefaultBackend != b.DefaultBackend && !a.DefaultBackend.Equal(b.DefaultBackend) {
		return false
	}
	return a.Annotations.Equal(b.Annotations)
}

//Equal compares two services, ignores statuses and old values
func (a *Service) Equal(b *Service) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if !a.Annotations.Equal(b.Annotations) {
		return false
	}
	if !a.Selector.Equal(b.Selector) {
		return false
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

//Equal compares two config maps, ignores statuses and old values
func (a *ConfigMap) Equal(b *ConfigMap) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if !a.Annotations.Equal(b.Annotations) {
		return false
	}
	return true
}

//Equal compares two secrets, ignores statuses and old values
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

//Equal checks if pods are equal
func (a *Endpoints) Equal(b *Endpoints) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Namespace != b.Namespace {
		return false
	}
	if a.Service != b.Service {
		return false
	}
	for port := range a.Ports {
		if _, ok := b.Ports[port]; !ok {
			return false
		}
		if a.Ports[port] != b.Ports[port] {
			return false
		}
	}
	if len(a.AddrRemain)+len(a.AddrsUsed) != len(b.AddrRemain)+len(b.AddrsUsed) {
		return false
	}
	for adr := range a.AddrRemain {
		if _, ok := b.AddrRemain[adr]; ok {
			continue
		}
		if _, ok := b.AddrsUsed[adr]; !ok {
			return false
		}
	}
	for adr := range a.AddrsUsed {
		if _, ok := b.AddrsUsed[adr]; ok {
			continue
		}
		if _, ok := b.AddrRemain[adr]; !ok {
			return false
		}
	}
	return true
}
