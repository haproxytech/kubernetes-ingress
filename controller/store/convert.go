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
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	extensions "k8s.io/api/extensions/v1beta1"
)

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
		rules[k8sRule.Host] = &IngressRule{
			Host:   k8sRule.Host,
			Paths:  paths,
			Status: "",
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
