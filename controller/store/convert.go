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

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// ConvertToIngress detects the interface{} provided by the SharedInformer and select
// the proper strategy to convert and return the resource as a store.Ingress struct
func ConvertToIngress(resource interface{}) (f *Ingress, err error) {
	switch t := resource.(type) {
	case *networkingv1beta1.Ingress:
		f = ingressNetworkingV1Beta1Strategy{obj: resource.(*networkingv1beta1.Ingress)}.Convert()
	case *extensionsv1beta1.Ingress:
		f = ingressExtensionsStrategy{obj: resource.(*extensionsv1beta1.Ingress)}.Convert()
	case *networkingv1.Ingress:
		f = ingressNetworkingV1Strategy{obj: resource.(*networkingv1.Ingress)}.Convert()
	default:
		err = fmt.Errorf("unrecognized type for: %T", t)
	}
	return
}

// ingressNetworkingV1Beta1Strategy is the Strategy implementation for converting an
// ingresses.networking.k8s.io/v1beta1 object into a store.Ingress resource.
type ingressNetworkingV1Beta1Strategy struct {
	obj *networkingv1beta1.Ingress
}

func getIngressClass(annotations map[string]string, specValue *string) string {
	// Giving priority to annotation due to backward compatibility as suggested
	// by Kuberentes documentation.
	if v, ok := annotations["kubernetes.io/ingress.class"]; ok {
		return v
	}
	// HAProxy Tech Ingress Controller allows also non prefixed annotations
	if v, ok := annotations["ingress.class"]; ok {
		return v
	}
	if specValue != nil {
		return *specValue
	}
	return ""
}

func (n ingressNetworkingV1Beta1Strategy) Convert() *Ingress {
	return &Ingress{
		APIVersion:  "networking.k8s.io/v1beta1",
		Namespace:   n.obj.GetNamespace(),
		Name:        n.obj.GetName(),
		Class:       getIngressClass(n.obj.GetAnnotations(), n.obj.Spec.IngressClassName),
		Annotations: ConvertToMapStringW(n.obj.GetAnnotations()),
		Rules: func(ingressRules []networkingv1beta1.IngressRule) map[string]*IngressRule {
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
						ExactPathMatch:    k8sPath.PathType != nil && *k8sPath.PathType == networkingv1beta1.PathTypeExact,
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
		}(n.obj.Spec.Rules),
		DefaultBackend: func(ingressBackend *networkingv1beta1.IngressBackend) *IngressPath {
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
		}(n.obj.Spec.Backend),
		TLS: func(ingressTLS []networkingv1beta1.IngressTLS) map[string]*IngressTLS {
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
		}(n.obj.Spec.TLS),
		Status: func() Status {
			if n.obj.ObjectMeta.GetDeletionTimestamp() != nil {
				return DELETED
			}
			return ADDED
		}(),
	}
}

// ingressExtensionsStrategy is the Strategy implementation for converting an
// ingresses.extensions/v1beta1 object into a store.Ingress resource.
type ingressExtensionsStrategy struct {
	obj *extensionsv1beta1.Ingress
}

func (e ingressExtensionsStrategy) Convert() *Ingress {
	return &Ingress{
		APIVersion:  "extensions/v1beta1",
		Namespace:   e.obj.GetNamespace(),
		Name:        e.obj.GetName(),
		Class:       getIngressClass(e.obj.GetAnnotations(), e.obj.Spec.IngressClassName),
		Annotations: ConvertToMapStringW(e.obj.GetAnnotations()),
		Rules: func(ingressRules []extensionsv1beta1.IngressRule) map[string]*IngressRule {
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
						ExactPathMatch:    k8sPath.PathType != nil && *k8sPath.PathType == extensionsv1beta1.PathTypeExact,
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
		}(e.obj.Spec.Rules),
		DefaultBackend: func(ingressBackend *extensionsv1beta1.IngressBackend) *IngressPath {
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
		}(e.obj.Spec.Backend),
		TLS: func(ingressTLS []extensionsv1beta1.IngressTLS) map[string]*IngressTLS {
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
		}(e.obj.Spec.TLS),
		Status: func() Status {
			if e.obj.ObjectMeta.GetDeletionTimestamp() != nil {
				return DELETED
			}
			return ADDED
		}(),
	}
}

// ingressNetworkingV1Strategy is the Strategy implementation for converting an
// ingresses.networking.k8s.io/v1 object into a store.Ingress resource.
type ingressNetworkingV1Strategy struct {
	obj *networkingv1.Ingress
}

func (n ingressNetworkingV1Strategy) Convert() *Ingress {
	return &Ingress{
		APIVersion:  "networking.k8s.io/v1",
		Namespace:   n.obj.GetNamespace(),
		Name:        n.obj.GetName(),
		Class:       getIngressClass(n.obj.GetAnnotations(), n.obj.Spec.IngressClassName),
		Annotations: ConvertToMapStringW(n.obj.GetAnnotations()),
		Rules: func(ingressRules []networkingv1.IngressRule) map[string]*IngressRule {
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
						ExactPathMatch:    k8sPath.PathType != nil && *k8sPath.PathType == networkingv1.PathTypeExact,
						ServiceName:       k8sPath.Backend.Service.Name,
						ServicePortInt:    int64(k8sPath.Backend.Service.Port.Number),
						ServicePortString: k8sPath.Backend.Service.Port.Name,
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
		}(n.obj.Spec.Rules),
		DefaultBackend: func(ingressBackend *networkingv1.IngressBackend) *IngressPath {
			if ingressBackend == nil {
				return nil
			}
			return &IngressPath{
				ServiceName:       ingressBackend.Service.Name,
				ServicePortInt:    int64(ingressBackend.Service.Port.Number),
				ServicePortString: ingressBackend.Service.Port.Name,
				IsDefaultBackend:  true,
				Status:            "",
			}
		}(n.obj.Spec.DefaultBackend),
		TLS: func(ingressTLS []networkingv1.IngressTLS) map[string]*IngressTLS {
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
		}(n.obj.Spec.TLS),
		Status: func() Status {
			if n.obj.ObjectMeta.GetDeletionTimestamp() != nil {
				return DELETED
			}
			return ADDED
		}(),
	}
}
