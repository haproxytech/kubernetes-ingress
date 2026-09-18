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
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
)

func (k *K8s) EventBackendCR(namespace, name string, data *v3.Backend) bool {
	ns := k.GetNamespace(namespace)
	key := namespace + "/" + name
	if data == nil {
		delete(ns.CRs.Backends, name)
		delete(k.BackendCRs, key)
		return true
	}
	ns.CRs.Backends[name] = &data.Spec
	state, known := k.BackendCRs[key]
	if !known {
		state = &BackendCRState{}
		k.BackendCRs[key] = state
	}
	if state.Generation != data.Generation {
		state.Disabled = false
	}
	state.Generation = data.Generation
	return true
}

// DisableBackendCR marks the current generation of a Backend custom resource as
// rejected by HAProxy. Returns that generation, and false for an unknown resource.
func (k *K8s) DisableBackendCR(namespace, name string) (generation int64, ok bool) {
	state, known := k.BackendCRs[namespace+"/"+name]
	if !known {
		return 0, false
	}
	state.Disabled = true
	return state.Generation, true
}

// BackendCRDisabled tells whether the stored generation of a Backend custom resource
// was rejected by HAProxy.
func (k *K8s) BackendCRDisabled(namespace, name string) bool {
	state, known := k.BackendCRs[namespace+"/"+name]
	return known && state.Disabled
}
