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

// PortEndpoints describes endpoints of a service port
type PortEndpoints struct {
	Port            int64
	DynUpdateFailed bool
	AddrCount       int
	AddrNew         map[string]struct{}
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

type Address struct {
	Address string
	Port    int64
}

// Auxiliary data about the state of the HAProxy for a service
type HAProxyConfig struct {
	HAProxySrvs     map[string]*[]*HAProxySrv      // port -> slice of HAProxySrvs
	NewAddresses    map[string]map[string]*Address // port -> set of Addresses
	BackendName     map[string]string              // port -> for runtime operations
	DynUpdateFailed map[string]bool
}

// Namespace is useful data from k8s structures about namespace
type Namespace struct {
	_             [0]int
	Name          string
	Relevant      bool
	Ingresses     map[string]*Ingress
	Endpoints     map[string]map[string]*Endpoints // service -> slice -> Endpoints
	Services      map[string]*Service
	Secret        map[string]*Secret
	HAProxyConfig map[string]*HAProxyConfig
	Status        Status
}

type IngressClass struct {
	APIVersion string
	Name       string
	Controller string
	Status     Status
}

// IngressPath is useful data from k8s structures about ingress path
type IngressPath struct {
	SvcName          string
	SvcPortInt       int64
	SvcPortString    string
	SvcPortResolved  *ServicePort
	Path             string
	PathTypeMatch    string
	IsDefaultBackend bool
	Status           Status
}

// IngressRule is useful data from k8s structures about ingress rule
type IngressRule struct {
	Host   string
	Paths  map[string]*IngressPath
	Status Status
}

// Ingress is useful data from k8s structures about ingress
type Ingress struct {
	// Required for K8s.UpdateIngressStatus to select proper versioned Client Set
	APIVersion     string
	Namespace      string
	Name           string
	Class          string
	Annotations    map[string]string
	Rules          map[string]*IngressRule
	DefaultBackend *IngressPath
	TLS            map[string]*IngressTLS
	Status         Status
}

// IngressTLS describes the transport layer security associated with an Ingress.
type IngressTLS struct {
	Host       string
	SecretName string
	Status     Status
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
