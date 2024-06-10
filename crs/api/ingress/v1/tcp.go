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
	"github.com/go-openapi/swag"
	"github.com/haproxytech/client-native/v5/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:metadata:annotations="haproxy.org/client-native=v5.1.4"

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
	models.Frontend `json:",inline"`
	Binds           []*models.Bind `json:"binds"`
}

type TCPModel struct {
	// +kubebuilder:validation:Required
	Name     string          `json:"name"`
	Frontend SectionFrontend `json:"frontend"`
	Service  TCPService      `json:"service"`
}

// TCPSpec defines the desired state of a TCPService
type TCPSpec []TCPModel

// DeepCopyInto deepcopying  the receiver into out. in must be non-nil.
func (a *TCPModel) DeepCopyInto(out *TCPModel) {
	*out = *a
	a.Frontend.DeepCopyInto(&out.Frontend)
	s, _ := a.Service.MarshalBinary()
	_ = out.Service.UnmarshalBinary(s)
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
	return swag.WriteJSON(s)
}

// UnmarshalBinary interface implementation
func (s *TCPService) UnmarshalBinary(b []byte) error {
	var res TCPService
	if err := swag.ReadJSON(b, &res); err != nil {
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
			if !value.Equal(*b.Frontend.Binds[i], opt...) {
				return false
			}
		}
	}

	if !a.Service.Equal(b.Service, opt...) {
		return false
	}
	return true
}

func (s TCPService) Equal(b TCPService, opt ...models.Options) bool {
	return s.Name == b.Name && s.Port == b.Port
}
