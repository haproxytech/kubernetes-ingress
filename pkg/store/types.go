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
	"fmt"
	"time"

	"github.com/haproxytech/client-native/v5/models"
	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
)

// ServicePort describes port of a service
type ServicePort struct {
	Name     string
	Protocol string
	Status   Status
	Port     int64
}

type HAProxySrv struct {
	// Srv disabled is srv with address ""
	Name     string
	Address  string
	Modified bool
	Port     int64
}

func (h *HAProxySrv) String() string {
	return fmt.Sprintf("%+v", *h)
}

// PortEndpoints describes endpoints of a service port
type PortEndpoints struct {
	Addresses map[string]struct{}
	Port      int64
}

// Endpoints describes endpoints of a service
type Endpoints struct {
	SliceName string
	Namespace string
	Service   string
	Ports     map[string]*PortEndpoints // Ports[portName]
	Status    Status
}

// PodEvent carries creation/deletion pod event.
type PodEvent struct {
	Status    Status
	Name      string
	Namespace string
}

// Service is useful data from k8s structures about service
type Service struct {
	Annotations map[string]string
	Namespace   string
	Name        string
	DNS         string
	Status      Status
	Ports       []ServicePort
	Addresses   []string // Used only for publish-service
	Faked       bool
}

// RuntimeBackend holds the runtime state of an HAProxy backend
type RuntimeBackend struct {
	Endpoints       PortEndpoints
	Name            string
	HAProxySrvs     []*HAProxySrv
	DynUpdateFailed bool
}

// Namespace is useful data from k8s structures about namespace
type Namespace struct {
	_               [0]int
	Secret          map[string]*Secret
	Ingresses       map[string]*Ingress
	Endpoints       map[string]map[string]*Endpoints // service -> sliceName -> Endpoints
	Services        map[string]*Service
	HAProxyRuntime  map[string]map[string]*RuntimeBackend // service -> portName -> Backend
	CRs             *CustomResources
	Gateways        map[string]*Gateway
	TCPRoutes       map[string]*TCPRoute
	ReferenceGrants map[string]*ReferenceGrant
	Labels          map[string]string
	Name            string
	Status          Status
	Relevant        bool
}

type CustomResources struct {
	Global     map[string]*models.Global
	Defaults   map[string]*models.Defaults
	LogTargets map[string]models.LogTargets
	Backends   map[string]*v1.BackendSpec
	TCPsPerCR  map[string]*TCPs // key is the TCP CR name
	AllTCPs    TCPResourceList
}

type IngressClass struct {
	APIVersion  string
	Name        string
	Controller  string
	Annotations map[string]string
	Status      Status // Used for store purpose
}

// IngressPath is useful data from k8s structures about ingress path
type IngressPath struct {
	SvcPortResolved  *ServicePort
	SvcNamespace     string
	SvcName          string
	SvcPortString    string
	Path             string
	PathTypeMatch    string
	SvcPortInt       int64
	IsDefaultBackend bool
}

// IngressRule is useful data from k8s structures about ingress rule
type IngressRule struct {
	Paths map[string]*IngressPath
	Host  string
}

// Ingress is useful data from k8s structures about ingress
type Ingress struct {
	IngressCore
	Status       Status // Used for store purpose
	Addresses    []string
	Ignored      bool // true if resource ignored because of non matching Controller Class
	ClassUpdated bool
	Faked        bool
}

// IngressTLS describes the transport layer security associated with an Ingress.
type IngressTLS struct {
	Host       string
	SecretName string
}

type ConfigMaps struct {
	Main         *ConfigMap
	TCPServices  *ConfigMap
	Errorfiles   *ConfigMap
	PatternFiles *ConfigMap
}

// ConfigMap is useful data from k8s structures about configmap
type ConfigMap struct {
	Annotations map[string]string
	Namespace   string
	Name        string
	Status      Status
	Loaded      bool
}

// Secret is useful data from k8s structures about secret
type Secret struct {
	Namespace string
	Name      string
	Data      map[string][]byte
	Status    Status
}

type IngressCore struct {
	Annotations    map[string]string
	Rules          map[string]*IngressRule
	DefaultBackend *IngressPath
	TLS            map[string]*IngressTLS
	APIVersion     string // Required for K8s.UpdateIngressStatus to select proper versioned Client Set
	Namespace      string
	Name           string
	Class          string
}

type GatewayClass struct {
	Description    *string
	Name           string
	ControllerName string
	Status         Status
	Generation     int64
}

type Gateway struct {
	Namespace        string
	Name             string
	GatewayClassName string
	Status           Status
	Listeners        []Listener
	Generation       int64
}
type Listener struct {
	Hostname      *string
	AllowedRoutes *AllowedRoutes
	Name          string
	Protocol      string
	GwNamespace   string
	GwName        string
	Port          int32
}

type AllowedRoutes struct {
	Namespaces *RouteNamespaces
	Kinds      []RouteGroupKind
}

type RouteGroupKind struct {
	Group *string
	Kind  string
}

type RouteNamespaces struct {
	From     *string
	Selector *LabelSelector
}

type TCPRoute struct {
	CreationTime time.Time
	Name         string
	Namespace    string
	Status       Status
	BackendRefs  []BackendRef
	ParentRefs   []ParentRef
	Generation   int64
}

type BackendRef struct {
	Namespace *string
	Port      *int32
	Weight    *int32
	Group     *string
	Kind      *string
	Name      string
}
type LabelSelector struct {
	MatchLabels      map[string]string
	MatchExpressions []LabelSelectorRequirement
}

type LabelSelectorRequirement struct {
	Key      string
	Operator string
	Values   []string
}

type TCPRoutes []TCPRoute

type ParentRef struct {
	Namespace   *string
	SectionName *string
	Port        *int32
	Group       string
	Kind        string
	Name        string
}

type ReferenceGrant struct {
	Namespace  string
	Name       string
	Status     Status
	From       []ReferenceGrantFrom
	To         []ReferenceGrantTo
	Generation int64
}

type ReferenceGrantFrom struct {
	Group     string
	Kind      string
	Namespace string
}

type ReferenceGrantTo struct {
	Name  *string
	Group string
	Kind  string
}
