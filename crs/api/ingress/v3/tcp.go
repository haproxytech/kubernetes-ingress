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

package v3

import (
	"github.com/go-openapi/swag/jsonutils"
	"github.com/haproxytech/client-native/v6/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:metadata:annotations="haproxy.org/client-native=v6.2.4"

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

type TCPModel struct {
	// +kubebuilder:validation:Required
	Name     string          `json:"name"`
	Frontend models.Frontend `json:"frontend"`
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

	f, _ := a.Frontend.MarshalBinary()
	out.Frontend = models.Frontend{}
	_ = out.Frontend.UnmarshalBinary(f)

	s, _ := a.Service.MarshalBinary()
	out.Service = TCPService{}
	_ = out.Service.UnmarshalBinary(s)

	if a.Services != nil {
		out.Services = make(TCPServices, len(a.Services))
		a.Services.DeepCopyInto(&out.Services)
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
	if !a.Frontend.Equal(b.Frontend, opt...) {
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
