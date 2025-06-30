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

package v3

import (
	"github.com/go-openapi/swag"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/validators"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=validationrules,singular=validationrules,scope=Namespaced
// +kubebuilder:metadata:annotations="haproxy.org/custom-annotations=v1.0.0"

// ValidationRules is a specification for a ValidationRules resource
type ValidationRules struct {
	Spec              ValidationRulesSpec `json:"spec"`
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

type ValidationRulesSpec struct {
	validators.Config `json:",inline"`
	Prefix            string `json:"prefix,omitempty" example:"company.xyz"`
}

// DeepCopyInto deepcopying  the receiver into out. in must be non-nil.
func (m *ValidationRulesSpec) DeepCopyInto(out *ValidationRulesSpec) {
	b, _ := m.MarshalBinary()
	_ = out.UnmarshalBinary(b)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MarshalBinary interface implementation
func (m *ValidationRulesSpec) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ValidationRulesSpec) UnmarshalBinary(b []byte) error {
	var res ValidationRulesSpec
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ValidationRulesList is a list of ValidationRules resources
type ValidationRulesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ValidationRules `json:"items"`
}
