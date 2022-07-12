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

package v1alpha2

import (
	"github.com/haproxytech/client-native/v3/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Backend is a specification for a Backend resource
type Backend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec BackendSpec `json:"spec"`
}

// BackendSpec defines the desired state of Backend
type BackendSpec struct {
	Config *models.Backend `json:"config"`
}

// DeepCopyInto deepcopying  the receiver into out. in must be non-nil.
func (in *BackendSpec) DeepCopyInto(out *BackendSpec) {
	*out = *in
	if in.Config != nil {
		b, _ := in.Config.MarshalBinary()
		_ = out.Config.UnmarshalBinary(b)
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackendList is a list of Backend resources
type BackendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Backend `json:"items"`
}
