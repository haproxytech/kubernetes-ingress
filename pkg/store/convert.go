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
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
)

//nolint:golint,stylecheck
const (
	NETWORKINGV1 = "networking.k8s.io/v1"

	PATH_TYPE_EXACT                   = "Exact"
	PATH_TYPE_PREFIX                  = "Prefix"
	PATH_TYPE_IMPLEMENTATION_SPECIFIC = "ImplementationSpecific"
)

// ConvertToIngress detects the interface{} provided by the SharedInformer and select
// the proper strategy to convert and return the resource as a store.Ingress struct
func ConvertToIngress(resource interface{}) (ingress *Ingress, err error) {
	switch t := resource.(type) {
	case *networkingv1.Ingress:
		ingress = ingressNetworkingV1Strategy{ig: resource.(*networkingv1.Ingress)}.ConvertIngress()
	default:
		err = fmt.Errorf("unrecognized type for: %T", t)
	}
	return ingress, err
}

func ConvertToIngressClass(resource interface{}) (ingress *IngressClass, err error) {
	switch t := resource.(type) {
	case *networkingv1.IngressClass:
		ingress = ingressNetworkingV1Strategy{class: resource.(*networkingv1.IngressClass)}.ConvertClass()
	default:
		err = fmt.Errorf("unrecognized type for: %T", t)
	}
	return ingress, err
}

// ingressNetworkingV1Strategy is the Strategy implementation for converting an
// ingresses.networking.k8s.io/v1 object into a store.Ingress resource.
type ingressNetworkingV1Strategy struct {
	ig    *networkingv1.Ingress
	class *networkingv1.IngressClass
}

func (n ingressNetworkingV1Strategy) ConvertIngress() *Ingress {
	ing := &Ingress{
		IngressCore: IngressCore{
			APIVersion:  NETWORKINGV1,
			Namespace:   n.ig.GetNamespace(),
			Name:        n.ig.GetName(),
			Class:       getIgClass(n.ig.Spec.IngressClassName),
			Annotations: CopyAnnotations(n.ig.GetAnnotations()),
			Rules: func(ingressRules []networkingv1.IngressRule) map[string]*IngressRule {
				rules := make(map[string]*IngressRule)
				for _, k8sRule := range ingressRules {
					paths := make(map[string]*IngressPath)
					if k8sRule.HTTP == nil {
						logger.Warningf("Ingress HTTP rules for [%s] does not exists", k8sRule.Host)
						continue
					}
					for _, k8sPath := range k8sRule.HTTP.Paths {
						var pathType string
						if k8sPath.PathType != nil {
							pathType = string(*k8sPath.PathType)
						}
						if k8sPath.Backend.Service == nil {
							logger.Errorf("backend in ingress '%s/%s' should have service but none found", n.ig.GetNamespace(), n.ig.GetName())
							continue
						}
						pathKey := pathType + "-" + k8sPath.Path + "-" + k8sPath.Backend.Service.Name + "-" + k8sPath.Backend.Service.Port.Name
						paths[pathKey] = &IngressPath{
							Path:          k8sPath.Path,
							PathTypeMatch: pathType,
							SvcNamespace:  n.ig.GetNamespace(),
						}
						if k8sPath.Backend.Service != nil {
							paths[pathKey].SvcName = k8sPath.Backend.Service.Name
							paths[pathKey].SvcPortInt = int64(k8sPath.Backend.Service.Port.Number)
							paths[pathKey].SvcPortString = k8sPath.Backend.Service.Port.Name
						}
					}
					if rule, ok := rules[k8sRule.Host]; ok {
						for path, ingressPath := range paths {
							rule.Paths[path] = ingressPath
						}
					} else {
						rules[k8sRule.Host] = &IngressRule{
							Host:  k8sRule.Host,
							Paths: paths,
						}
					}
				}
				return rules
			}(n.ig.Spec.Rules),
			DefaultBackend: func(ingressBackend *networkingv1.IngressBackend) *IngressPath {
				if ingressBackend == nil {
					return nil
				}
				ingPath := &IngressPath{
					SvcNamespace:     n.ig.GetNamespace(),
					IsDefaultBackend: true,
				}
				if ingressBackend.Service != nil {
					ingPath.SvcName = ingressBackend.Service.Name
					ingPath.SvcPortInt = int64(ingressBackend.Service.Port.Number)
					ingPath.SvcPortString = ingressBackend.Service.Port.Name
				}
				return ingPath
			}(n.ig.Spec.DefaultBackend),
			TLS: func(ingressTLS []networkingv1.IngressTLS) map[string]*IngressTLS {
				tls := make(map[string]*IngressTLS)
				for _, k8sTLS := range ingressTLS {
					for _, host := range k8sTLS.Hosts {
						tls[host] = &IngressTLS{
							Host:       host,
							SecretName: k8sTLS.SecretName,
						}
					}
				}
				return tls
			}(n.ig.Spec.TLS),
		},
	}
	addresses := []string{}
	for _, ingLoadBalancer := range n.ig.Status.LoadBalancer.Ingress {
		address := ingLoadBalancer.IP
		if ingLoadBalancer.Hostname != "" {
			address = ingLoadBalancer.Hostname
		}
		addresses = append(addresses, address)
	}
	ing.Addresses = addresses
	return ing
}

func (n ingressNetworkingV1Strategy) ConvertClass() *IngressClass {
	annotations := make(map[string]string, len(n.class.Annotations))
	for key, value := range n.class.Annotations {
		annotations[key] = value
	}
	return &IngressClass{
		APIVersion:  NETWORKINGV1,
		Name:        n.class.GetName(),
		Controller:  n.class.Spec.Controller,
		Annotations: annotations,
	}
}

func getIgClass(className *string) string {
	if className == nil {
		return ""
	}
	return *className
}

// CopyAnnotations returns a copy of annotations map and removes prefixe from annotations name
func CopyAnnotations(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for name, value := range in {
		split := strings.SplitN(name, "/", 2)
		out[split[len(split)-1]] = value
	}
	return out
}
