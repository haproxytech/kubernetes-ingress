// Copyright 2019 HAProxy Technologies
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package v1

import (
	"github.com/haproxytech/client-native/v5/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:metadata:annotations="haproxy.org/client-native=v5.1.18-0.20250618120639-fbea0cb7be62"

// Defaults is a specification for a Defaults resource
type Defaults struct {
	Spec              DefaultsSpec `json:"spec"`
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// DefaultsSpec defines the desired state of Defaults
type DefaultsSpec struct {
	Config *models.Defaults `json:"config"`
}

// DeepCopyInto deepcopying  the receiver into out. in must be non-nil.
func (in *DefaultsSpec) DeepCopyInto(out *DefaultsSpec) {
	*out = *in
	if in.Config != nil {
		b, _ := in.Config.MarshalBinary()
		_ = out.Config.UnmarshalBinary(b)
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DefaultsList is a list of Defaults resources
type DefaultsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Defaults `json:"items"`
}
