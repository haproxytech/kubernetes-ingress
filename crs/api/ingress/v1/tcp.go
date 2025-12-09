// Copyright 2024 HAProxy Technologies
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
	"github.com/go-openapi/swag/jsonutils"
	"github.com/haproxytech/client-native/v5/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:metadata:annotations="haproxy.org/client-native=v5.1.18-0.20250618120639-fbea0cb7be62"

// TCP is a specification for a TCP resource
type TCP struct {
	Spec              TCPSpec `json:"spec"`
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

type TCPService struct {
	Name string `json:"name"`
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Minimum=1
	Port int `json:"port"`
}

type SectionFrontend struct {
	models.Frontend       `json:",inline"`
	Acls                  models.Acls                  `json:"acl_list,omitempty"`
	Binds                 []*models.Bind               `json:"binds"`
	BackendSwitchingRules models.BackendSwitchingRules `json:"backend_switching_rule_list,omitempty"`
	Captures              models.Captures              `json:"capture_list,omitempty"`
	Filters               models.Filters               `json:"filter_list,omitempty"`
	LogTargets            models.LogTargets            `json:"log_target_list,omitempty"`
	TCPRequestRules       models.TCPRequestRules       `json:"tcp_request_rule_list,omitempty"`
}

type TCPModel struct {
	// +kubebuilder:validation:Required
	Name     string          `json:"name"`
	Frontend SectionFrontend `json:"frontend"`
	// Service defines the name of the default service (default_backend)
	Service TCPService `json:"service"`
	// Services defines additional services for additional backends
	Services TCPServices `json:"services,omitempty"`
}

type TCPServices []*TCPService

// TCPSpec defines the desired state of a TCPService
type TCPSpec []TCPModel

// DeepCopyInto deepcopying  the receiver into out. in must be non-nil.
func (a *TCPModel) DeepCopyInto(out *TCPModel) {
	*out = *a
	a.Frontend.DeepCopyInto(&out.Frontend)
	s, _ := a.Service.MarshalBinary()
	_ = out.Service.UnmarshalBinary(s)

	if a.Services != nil {
		a.Services.DeepCopyInto(&out.Services)
	}
}

func (a *SectionFrontend) DeepCopyInto(out *SectionFrontend) {
	*out = *a

	b, _ := a.Frontend.MarshalBinary()
	_ = out.Frontend.UnmarshalBinary(b)
	if a.Binds != nil {
		out.Binds = make([]*models.Bind, len(a.Binds))
		for i, v := range a.Binds {
			b, _ := v.MarshalBinary()
			out.Binds[i] = &models.Bind{}
			_ = out.Binds[i].UnmarshalBinary(b)
		}
	}

	if a.Acls != nil {
		out.Acls = make(models.Acls, len(a.Acls))
		for i, v := range a.Acls {
			b, _ := v.MarshalBinary()
			out.Acls[i] = &models.ACL{}
			_ = out.Acls[i].UnmarshalBinary(b)
		}
	}

	if len(a.BackendSwitchingRules) > 0 {
		out.BackendSwitchingRules = make([]*models.BackendSwitchingRule, len(a.BackendSwitchingRules))
		for i, v := range a.BackendSwitchingRules {
			b, _ := v.MarshalBinary()
			out.BackendSwitchingRules[i] = &models.BackendSwitchingRule{}
			_ = out.BackendSwitchingRules[i].UnmarshalBinary(b)
		}
	}

	if len(a.Captures) > 0 {
		out.Captures = make([]*models.Capture, len(a.Captures))
		for i, v := range a.Captures {
			b, _ := v.MarshalBinary()
			out.Captures[i] = &models.Capture{}
			_ = out.Captures[i].UnmarshalBinary(b)
		}
	}

	if len(a.Filters) > 0 {
		out.Filters = make([]*models.Filter, len(a.Filters))
		for i, v := range a.Filters {
			b, _ := v.MarshalBinary()
			out.Filters[i] = &models.Filter{}
			_ = out.Filters[i].UnmarshalBinary(b)
		}
	}

	if len(a.LogTargets) > 0 {
		out.LogTargets = make([]*models.LogTarget, len(a.LogTargets))
		for i, v := range a.LogTargets {
			b, _ := v.MarshalBinary()
			out.LogTargets[i] = &models.LogTarget{}
			_ = out.LogTargets[i].UnmarshalBinary(b)
		}
	}

	if len(a.TCPRequestRules) > 0 {
		out.TCPRequestRules = make([]*models.TCPRequestRule, len(a.TCPRequestRules))
		for i, v := range a.TCPRequestRules {
			b, _ := v.MarshalBinary()
			out.TCPRequestRules[i] = &models.TCPRequestRule{}
			_ = out.TCPRequestRules[i].UnmarshalBinary(b)
		}
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TCPList is a list of TCP resources
type TCPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TCP `json:"items"`
}

// MarshalBinary interface implementation
func (s *TCPService) MarshalBinary() ([]byte, error) {
	if s == nil {
		return nil, nil
	}
	return jsonutils.WriteJSON(s)
}

// UnmarshalBinary interface implementation
func (s *TCPService) UnmarshalBinary(b []byte) error {
	var res TCPService
	if err := jsonutils.ReadJSON(b, &res); err != nil {
		return err
	}
	*s = res
	return nil
}

func (a TCPModel) Equal(b TCPModel, opt ...models.Options) bool {
	if a.Name != b.Name {
		return false
	}
	if !a.Frontend.Frontend.Equal(b.Frontend.Frontend, opt...) {
		return false
	}
	if (a.Frontend.Binds == nil && b.Frontend.Binds != nil) || (a.Frontend.Binds != nil && b.Frontend.Binds == nil) {
		return false
	}
	if a.Frontend.Binds != nil && b.Frontend.Binds != nil {
		if len(a.Frontend.Binds) != len(b.Frontend.Binds) {
			return false
		}
		for i, value := range a.Frontend.Binds {
			if (value == nil && b.Frontend.Binds[i] != nil) || (value != nil && b.Frontend.Binds[i] == nil) {
				return false
			}
			if value != nil &&
				b.Frontend.Binds[i] != nil &&
				!value.Equal(*b.Frontend.Binds[i], opt...) {
				return false
			}
		}
	}

	if !a.Frontend.Acls.Equal(b.Frontend.Acls, models.Options{
		NilSameAsEmpty: true,
	}) {
		return false
	}

	if !a.Frontend.BackendSwitchingRules.Equal(b.Frontend.BackendSwitchingRules, models.Options{
		NilSameAsEmpty: true,
	}) {
		return false
	}

	if !a.Frontend.Captures.Equal(b.Frontend.Captures, models.Options{
		NilSameAsEmpty: true,
	}) {
		return false
	}

	if !a.Frontend.Filters.Equal(b.Frontend.Filters, models.Options{
		NilSameAsEmpty: true,
	}) {
		return false
	}

	if !a.Frontend.LogTargets.Equal(b.Frontend.LogTargets, models.Options{
		NilSameAsEmpty: true,
	}) {
		return false
	}

	if !a.Frontend.TCPRequestRules.Equal(b.Frontend.TCPRequestRules, models.Options{
		NilSameAsEmpty: true,
	}) {
		return false
	}

	if !a.Service.Equal(b.Service, opt...) {
		return false
	}

	if (a.Services == nil && b.Services != nil) || (a.Services != nil && b.Services == nil) {
		return false
	}
	if a.Services != nil && b.Services != nil {
		if len(a.Services) != len(b.Services) {
			return false
		}
		for i, value := range a.Services {
			if (value == nil && b.Services[i] != nil) || (value != nil && b.Services[i] == nil) {
				return false
			}
			if value != nil &&
				b.Services[i] != nil &&
				!value.Equal(*b.Services[i], opt...) {
				return false
			}
		}
	}

	return true
}

func (s TCPService) Equal(b TCPService, opt ...models.Options) bool {
	return s.Name == b.Name && s.Port == b.Port
}
