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
// +kubebuilder:metadata:annotations="haproxy.org/client-native=v5.1.3-0.20240213005611-75890279f890"
// +kubebuilder:validation:XValidation:rule="!has(self.spec.config.default_path)", message="spec.config.default_path is set by ingress controller internally"
// +kubebuilder:validation:XValidation:rule="!has(self.spec.config.master__dash__worker)", message="spec.config.master-worker is set by ingress controller internally"
// +kubebuilder:validation:XValidation:rule="!has(self.spec.config.pidfile)", message="spec.config.pidfile is set by ingress controller internally"
// +kubebuilder:validation:XValidation:rule="!has(self.spec.config.localpeer)", message="spec.config.localpeer is set by ingress controller internally"

// Global is a specification for a Global resource
type Global struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GlobalSpec `json:"spec"`
}

// GlobalSpec defines the desired state of Global
type GlobalSpec struct {
	Config     *models.Global    `json:"config"`
	LogTargets models.LogTargets `json:"log_targets,omitempty"` //nolint:tagliatelle
}

// DeepCopyInto deepcopying  the receiver into out. in must be non-nil.
func (in *GlobalSpec) DeepCopyInto(out *GlobalSpec) {
	if in.Config != nil {
		b, _ := in.Config.MarshalBinary()
		_ = out.Config.UnmarshalBinary(b)
	}
	if in.LogTargets != nil {
		out.LogTargets = make([]*models.LogTarget, len(in.LogTargets))
		for i, v := range in.LogTargets {
			b, _ := v.MarshalBinary()
			out.LogTargets[i] = &models.LogTarget{}
			_ = out.LogTargets[i].UnmarshalBinary(b)
		}
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalList is a list of Global resources
type GlobalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Global `json:"items"`
}
