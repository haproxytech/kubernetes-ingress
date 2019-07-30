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

package main

//ServicePort describes port of a service
type ServicePort struct {
	Name        string
	Protocol    string
	ServicePort int64
	TargetPort  int64
	Status      Status
}

type ServicePorts []*ServicePort

type EndpointIP struct {
	IP          string
	Name        string
	HAProxyName string
	Disabled    bool
	Status      Status
}

type EndpointIPs map[string]*EndpointIP

//Endpoints is usefull data from k8s structures about Endpoints
type Endpoints struct {
	Namespace   string
	Service     StringW
	BackendName string
	Ports       *ServicePorts
	Addresses   *EndpointIPs
	Status      Status
}

//Service is usefull data from k8s structures about service
type Service struct {
	Namespace   string
	Name        string
	ClusterIP   string
	ExternalIP  string
	Ports       []ServicePort
	Annotations MapStringW
	Selector    MapStringW
	Status      Status
}

//Namespace is usefull data from k8s structures about namespace
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

//IngressPath is usefull data from k8s structures about ingress path
type IngressPath struct {
	ServiceName       string
	ServicePortInt    int64
	ServicePortString string
	Path              string
	PathIndex         int
	Status            Status
}

//IngressRule is usefull data from k8s structures about ingress rule
type IngressRule struct {
	Host   string
	Paths  map[string]*IngressPath
	Status Status
}

//Ingress is usefull data from k8s structures about ingress
type Ingress struct {
	Namespace   string
	Name        string
	Annotations MapStringW
	Rules       map[string]*IngressRule
	Status      Status
}

//ConfigMap is usefull data from k8s structures about configmap
type ConfigMap struct {
	Namespace   string
	Name        string
	Annotations MapStringW
	Status      Status
}

//Secret is usefull data from k8s structures about secret
type Secret struct {
	Namespace string
	Name      string
	Data      map[string][]byte
	Status    Status
}
