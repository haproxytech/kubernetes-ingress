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

	"github.com/haproxytech/client-native/v3/models"
)

// ServicePort describes port of a service
type ServicePort struct {
	Name     string
	Protocol string
	Port     int64
	Status   Status
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
	Port      int64
	Addresses map[string]struct{}
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
	Created bool
}

// Service is useful data from k8s structures about service
type Service struct {
	Namespace   string
	Name        string
	Ports       []ServicePort
	Addresses   []string // Used only for publish-service
	DNS         string
	Annotations map[string]string
	Status      Status
}

// RuntimeBackend holds the runtime state of an HAProxy backend
type RuntimeBackend struct {
	Name            string
	HAProxySrvs     []*HAProxySrv
	Endpoints       PortEndpoints
	DynUpdateFailed bool
}

// Namespace is useful data from k8s structures about namespace
type Namespace struct {
	_               [0]int
	Name            string
	Relevant        bool
	Ingresses       map[string]*Ingress
	Endpoints       map[string]map[string]*Endpoints // service -> sliceName -> Endpoints
	Services        map[string]*Service
	Secret          map[string]*Secret
	HAProxyRuntime  map[string]map[string]*RuntimeBackend // service -> portName -> Backend
	CRs             *CustomResources
	Gateways        map[string]*Gateway
	TCPRoutes       map[string]*TCPRoute
	ReferenceGrants map[string]*ReferenceGrant
	Labels          map[string]string
	Status          Status
}

type CustomResources struct {
	Global     map[string]*models.Global
	Defaults   map[string]*models.Defaults
	LogTargets map[string]models.LogTargets
	Backends   map[string]*models.Backend
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
	SvcNamespace     string
	SvcName          string
	SvcPortInt       int64
	SvcPortString    string
	SvcPortResolved  *ServicePort
	Path             string
	PathTypeMatch    string
	IsDefaultBackend bool
}

// IngressRule is useful data from k8s structures about ingress rule
type IngressRule struct {
	Host  string
	Paths map[string]*IngressPath
}

// Ingress is useful data from k8s structures about ingress
type Ingress struct {
	IngressCore
	Ignored   bool   // true if resource ignored because of non matching Controller Class
	Status    Status // Used for store purpose
	Addresses []string
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
	Namespace   string
	Name        string
	Loaded      bool
	Annotations map[string]string
	Status      Status
}

// Secret is useful data from k8s structures about secret
type Secret struct {
	Namespace string
	Name      string
	Data      map[string][]byte
	Status    Status
}

type IngressCore struct {
	// Required for K8s.UpdateIngressStatus to select proper versioned Client Set
	APIVersion     string
	Namespace      string
	Name           string
	Class          string
	Annotations    map[string]string
	Rules          map[string]*IngressRule
	DefaultBackend *IngressPath
	TLS            map[string]*IngressTLS
}

type GatewayClass struct {
	Name           string
	ControllerName string
	Description    *string
	Generation     int64
	Status         Status
}

type Gateway struct {
	Namespace        string
	Name             string
	GatewayClassName string
	Listeners        []Listener
	Generation       int64
	Status           Status
}
type Listener struct {
	Name          string
	Port          int32
	Protocol      string
	Hostname      *string
	AllowedRoutes *AllowedRoutes
	GwNamespace   string
	GwName        string
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
	Name         string
	Namespace    string
	BackendRefs  []BackendRef
	ParentRefs   []ParentRef
	CreationTime time.Time
	Generation   int64
	Status       Status
}

type BackendRef struct {
	Name      string
	Namespace *string
	Port      *int32
	Weight    *int32
	Group     *string
	Kind      *string
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
	Group       string
	Kind        string
	Namespace   *string
	Name        string
	SectionName *string
	Port        *int32
}

type ReferenceGrant struct {
	Namespace  string
	Name       string
	From       []ReferenceGrantFrom
	To         []ReferenceGrantTo
	Generation int64
	Status     Status
}

type ReferenceGrantFrom struct {
	Group     string
	Kind      string
	Namespace string
}

type ReferenceGrantTo struct {
	Group string
	Kind  string
	Name  *string
}

const (
	TCPProtocolType string = "TCP"
)
