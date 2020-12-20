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

package controller

import (
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	extensions "k8s.io/api/extensions/v1beta1"
)

const (
	FrontendHTTP  = "http"
	FrontendHTTPS = "https"
	FrontendSSL   = "ssl"
)

var (
	HAProxyCFG      string
	HAProxyCertDir  string
	HAProxyStateDir string
	HAProxyMapDir   string
	HAProxyPIDFile  string
	TransactionDir  string
)

//ServicePort describes port of a service
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
}

//PortEndpoints describes endpionts of a service port
type PortEndpoints struct {
	Port        int64
	BackendName string
	AddrCount   int
	AddrNew     map[string]struct{}
	HAProxySrvs []*HAProxySrv
}

//Endpoints describes endpoints of a service
type Endpoints struct {
	Namespace string
	Service   StringW
	Ports     map[string]*PortEndpoints
	Status    Status
}

//Service is usefull data from k8s structures about service
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
	IsTCPService      bool
	IsSSLPassthrough  bool
	IsDefaultBackend  bool
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

//ConvertIngressRules converts data from kubernetes format
func ConvertIngressRules(ingressRules []extensions.IngressRule) map[string]*IngressRule {
	rules := make(map[string]*IngressRule)
	for _, k8sRule := range ingressRules {
		paths := make(map[string]*IngressPath)
		if k8sRule.HTTP == nil {
			logger := utils.GetLogger()
			logger.Warningf("Ingress HTTP rules for [%s] does not exists", k8sRule.Host)
			continue
		}
		for _, k8sPath := range k8sRule.HTTP.Paths {
			paths[k8sPath.Path] = &IngressPath{
				Path:              k8sPath.Path,
				ServiceName:       k8sPath.Backend.ServiceName,
				ServicePortInt:    int64(k8sPath.Backend.ServicePort.IntValue()),
				ServicePortString: k8sPath.Backend.ServicePort.StrVal,
				Status:            "",
			}
		}
		if rule, ok := rules[k8sRule.Host]; ok {
			for path, ingressPath := range paths {
				rule.Paths[path] = ingressPath
			}
		} else {
			rules[k8sRule.Host] = &IngressRule{
				Host:   k8sRule.Host,
				Paths:  paths,
				Status: "",
			}
		}
	}
	return rules
}

//ConvertIngressRules converts data from kubernetes format
func ConvertIngressTLS(ingressTLS []extensions.IngressTLS) map[string]*IngressTLS {
	tls := make(map[string]*IngressTLS)
	for _, k8sTLS := range ingressTLS {
		for _, host := range k8sTLS.Hosts {
			tls[host] = &IngressTLS{
				Host: host,
				SecretName: StringW{
					Value: k8sTLS.SecretName,
				},
				Status: EMPTY,
			}
		}
	}
	return tls
}

func ConvertIngressBackend(ingressBackend *extensions.IngressBackend) *IngressPath {
	if ingressBackend == nil {
		return nil
	}
	return &IngressPath{
		ServiceName:       ingressBackend.ServiceName,
		ServicePortInt:    int64(ingressBackend.ServicePort.IntValue()),
		ServicePortString: ingressBackend.ServicePort.StrVal,
		IsDefaultBackend:  true,
		Status:            "",
	}
}
