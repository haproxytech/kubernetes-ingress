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

package rc

import (
	"strings"
	"sync"
)

type (
	ResourceType           string
	OwnerKey               string // = resource type:[namespace]/name = k8s resource key
	HaproxyCfgResourceName string // = Haproxy cfg resource name
)

//nolint:golint,stylecheck
const (
	// ResourceType values
	TCP_CR        ResourceType = "tcp-cr"
	TCP_CONFIGMAP ResourceType = "tcp-configmap"
)

type (
	Owners map[OwnerKey]struct{}
	Owned  map[HaproxyCfgResourceName]struct{}
	Owner  struct {
		resourceType ResourceType
		namespace    string
		name         string
	}
)

type ResourceCounter struct {
	mu     sync.Mutex
	owners map[HaproxyCfgResourceName]Owners
	owned  map[OwnerKey]Owned
}

func NewResourceCounter() *ResourceCounter {
	rc := ResourceCounter{
		owners: map[HaproxyCfgResourceName]Owners{},
		owned:  map[OwnerKey]Owned{},
	}
	return &rc
}

func NewOwner(rtype ResourceType, namespace, name string) Owner {
	return Owner{
		resourceType: rtype,
		namespace:    namespace,
		name:         name,
	}
}

func (o *Owner) Key() OwnerKey {
	var sb strings.Builder
	sb.WriteString(string(o.resourceType))
	sb.WriteString(":")
	if o.namespace != "" {
		sb.WriteString(o.namespace)
		sb.WriteString("/")
	}
	sb.WriteString(o.name)
	return OwnerKey(sb.String())
}

func (rc *ResourceCounter) AddOwner(haproxyResourceName HaproxyCfgResourceName, owner Owner) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	ownerKey := owner.Key()
	if _, ok := rc.owners[haproxyResourceName]; !ok {
		rc.owners[haproxyResourceName] = Owners{}
	}
	if _, ok := rc.owned[ownerKey]; !ok {
		rc.owned[ownerKey] = Owned{}
	}
	rc.owned[ownerKey][haproxyResourceName] = struct{}{}
	rc.owners[haproxyResourceName][ownerKey] = struct{}{}
}

func (rc *ResourceCounter) GetOwners(name HaproxyCfgResourceName) (Owners, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	v, ok := rc.owners[name]
	return v, ok
}

func (rc *ResourceCounter) GetOwned(owner Owner) Owned {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.owned[owner.Key()]
}

func (rc *ResourceCounter) RemoveOwnerForCfgResource(cfgResourceName HaproxyCfgResourceName, owner Owner) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	delete(rc.owners[cfgResourceName], owner.Key())
}

func (rc *ResourceCounter) RemoveOwner(owner Owner) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	ownerKey := owner.Key()
	for cfgResourceName := range rc.owned[ownerKey] {
		delete(rc.owners[cfgResourceName], ownerKey)
		if _, ok := rc.owners[cfgResourceName]; ok && len(rc.owners[cfgResourceName]) == 0 {
			delete(rc.owners, cfgResourceName)
		}
	}
	delete(rc.owned, ownerKey)
}

func (rc *ResourceCounter) HasOwners(name HaproxyCfgResourceName) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	v, ok := rc.owners[name]
	return ok && len(v) > 0
}

func (rc *ResourceCounter) Clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	clear(rc.owners)
	clear(rc.owned)
}
