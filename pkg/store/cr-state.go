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

import "fmt"

// CRKind names a custom resource kind the controller renders sections from.
type CRKind string

const (
	CRKindGlobal   CRKind = "Global"
	CRKindDefaults CRKind = "Defaults"
	CRKindFrontend CRKind = "Frontend"
	CRKindBackend  CRKind = "Backend"
	CRKindTCP      CRKind = "TCP"
)

// CRRef identifies one custom resource.
type CRRef struct {
	Kind      CRKind
	Namespace string
	Name      string
}

func (r CRRef) String() string {
	return fmt.Sprintf("%s '%s/%s'", r.Kind, r.Namespace, r.Name)
}

// CRState is what the store knows of a custom resource beyond its spec.
//
// HAProxy validates the whole configuration at once, so a resource it rejects blocks
// every other change until fixed. Disabled marks the stored Generation as rejected;
// a new generation clears it so the resource gets retried once edited.
type CRState struct {
	Generation int64
	Disabled   bool
}

// trackCR records the generation of a resource seen in an event.
// Reports whether that put a disabled resource back under test.
func (k *K8s) trackCR(ref CRRef, generation int64) (reenabled bool) {
	state, known := k.CRStates[ref]
	if !known {
		state = &CRState{}
		k.CRStates[ref] = state
	}
	if state.Generation != generation {
		reenabled = state.Disabled
		state.Disabled = false
	}
	state.Generation = generation
	return reenabled
}

func (k *K8s) forgetCR(ref CRRef) {
	delete(k.CRStates, ref)
}

// DisableCR marks the current generation of a custom resource as rejected by
// HAProxy. Returns that generation, and false for an unknown resource.
func (k *K8s) DisableCR(ref CRRef) (generation int64, ok bool) {
	state, known := k.CRStates[ref]
	if !known {
		return 0, false
	}
	state.Disabled = true
	return state.Generation, true
}

// CRDisabled tells whether the stored generation of a custom resource was rejected by HAProxy.
func (k *K8s) CRDisabled(ref CRRef) bool {
	state, known := k.CRStates[ref]
	return known && state.Disabled
}
