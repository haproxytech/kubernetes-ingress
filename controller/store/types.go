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

//ServicePort describes port of a service
type ServicePort struct {
	Name     string
	Protocol string
	Port     int64
	Status   Status
}

type HAProxySrv struct {
	IP       string
	Disabled bool
	Modified bool
}

//Endpoints is useful data from k8s structures about Endpoints
type Endpoints struct {
	Namespace   string
	Service     StringW
	BackendName string
	Ports       map[string]int64
	Addresses   map[string]struct{}
	HAProxySrvs map[string]*HAProxySrv
	Status      Status
}

//Service is useful data from k8s structures about service
type Service struct {
	Namespace   string
	Name        string
	Ports       []ServicePort
	Addresses   []string //Used only for publish-service
	DNS         string
	Annotations MapStringW
	Selector    MapStringW
	Status      Status
}

//Namespace is useful data from k8s structures about namespace
type Namespace struct {
	_         [0]int
	Name      string
	Relevant  bool
	Ingresses map[string]*Ingress
	Endpoints map[string]*Endpoints
	Services  map[string]*Service
	Secret    map[string]*Secret
	Status    Status
}

//IngressPath is useful data from k8s structures about ingress path
type IngressPath struct {
	ServiceName       string
	ServicePortInt    int64
	ServicePortString string
	TargetPort        int64
	Path              string
	ExactPathMatch    bool
	IsTCPService      bool
	IsSSLPassthrough  bool
	IsDefaultBackend  bool
	Status            Status
}

//IngressRule is useful data from k8s structures about ingress rule
type IngressRule struct {
	Host   string
	Paths  map[string]*IngressPath
	Status Status
}

//Ingress is useful data from k8s structures about ingress
type Ingress struct {
	// Required for K8s.UpdateIngressStatus to select proper versioned Client Set
	APIVersion     string
	Namespace      string
	Name           string
	Annotations    MapStringW
	Rules          map[string]*IngressRule
	DefaultBackend *IngressPath
	TLS            map[string]*IngressTLS
	Status         Status
}

// IngressTLS describes the transport layer security associated with an Ingress.
type IngressTLS struct {
	Host       string
	SecretName StringW
	Status     Status
}

//ConfigMap is useful data from k8s structures about configmap
type ConfigMap struct {
	Namespace   string
	Name        string
	Annotations MapStringW
	Status      Status
}

//Secret is useful data from k8s structures about secret
type Secret struct {
	Namespace string
	Name      string
	Data      map[string][]byte
	Status    Status
}
