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
// +kubebuilder:metadata:annotations="haproxy.org/client-native=v5.1.11"

// Backend is a specification for a Backend resource
type Backend struct {
	Spec              BackendSpec `json:"spec"`
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// BackendSpec defines the desired state of Backend
type BackendSpec struct {
	Config       *models.Backend         `json:"config"`
	Acls         models.Acls             `json:"acls,omitempty"`
	HTTPRequests models.HTTPRequestRules `json:"http-requests,omitempty"`
}

// DeepCopyInto deepcopying  the receiver into out. in must be non-nil.
func (in *BackendSpec) DeepCopyInto(out *BackendSpec) {
	*out = *in
	if in.Config != nil {
		b, _ := in.Config.MarshalBinary()
		_ = out.Config.UnmarshalBinary(b)
	}
	if in.Acls != nil {
		out.Acls = make(models.Acls, len(in.Acls))
		for i, v := range in.Acls {
			b, _ := v.MarshalBinary()
			out.Acls[i] = &models.ACL{}
			_ = out.Acls[i].UnmarshalBinary(b)
		}
	}

	if in.HTTPRequests != nil {
		out.HTTPRequests = make(models.HTTPRequestRules, len(in.HTTPRequests))
		for i, v := range in.HTTPRequests {
			b, _ := v.MarshalBinary()
			out.HTTPRequests[i] = &models.HTTPRequestRule{}
			_ = out.HTTPRequests[i].UnmarshalBinary(b)
		}
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackendList is a list of Backend resources
type BackendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Backend `json:"items"`
}
