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
)

//nolint:golint,stylecheck
const (
	NETWORKINGV1BETA1 = "networking.k8s.io/v1beta1"
	EXTENSIONSV1BETA1 = "extensions/v1beta1"
	NETWORKINGV1      = "networking.k8s.io/v1"

	PATH_TYPE_EXACT                   = "Exact"
	PATH_TYPE_PREFIX                  = "Prefix"
	PATH_TYPE_IMPLEMENTATION_SPECIFIC = "ImplementationSpecific"
)

// ConvertToIngress detects the interface{} provided by the SharedInformer and select
// the proper strategy to convert and return the resource as a store.Ingress struct
func ConvertToIngress(resource interface{}) (ingress *Ingress, err error) {
	switch t := resource.(type) {
	case *networkingv1beta1.Ingress:
		ingress = ingressNetworkingV1Beta1Strategy{ig: resource.(*networkingv1beta1.Ingress)}.ConvertIngress()
		for _, rule := range ingress.Rules {
			for _, path := range rule.Paths {
				if path.PathTypeMatch == "" {
					path.PathTypeMatch = PATH_TYPE_IMPLEMENTATION_SPECIFIC
				}
			}
		}
	case *extensionsv1beta1.Ingress:
		ingress = ingressExtensionsStrategy{ig: resource.(*extensionsv1beta1.Ingress)}.ConvertIngress()
		for _, rule := range ingress.Rules {
			for _, path := range rule.Paths {
				if path.PathTypeMatch == "" {
					path.PathTypeMatch = PATH_TYPE_IMPLEMENTATION_SPECIFIC
				}
			}
		}
	case *networkingv1.Ingress:
		ingress = ingressNetworkingV1Strategy{ig: resource.(*networkingv1.Ingress)}.ConvertIngress()
	default:
		err = fmt.Errorf("unrecognized type for: %T", t)
	}
	return
}

func ConvertToIngressClass(resource interface{}) (ingress *IngressClass, err error) {
	switch t := resource.(type) {
	case *networkingv1beta1.IngressClass:
		ingress = ingressNetworkingV1Beta1Strategy{class: resource.(*networkingv1beta1.IngressClass)}.ConvertClass()
	case *networkingv1.IngressClass:
		ingress = ingressNetworkingV1Strategy{class: resource.(*networkingv1.IngressClass)}.ConvertClass()
	default:
		err = fmt.Errorf("unrecognized type for: %T", t)
	}
	return
}

// ingressNetworkingV1Beta1Strategy is the Strategy implementation for converting an
// ingresses.networking.k8s.io/v1beta1 object into a store.Ingress resource.
type ingressNetworkingV1Beta1Strategy struct {
	ig    *networkingv1beta1.Ingress
	class *networkingv1beta1.IngressClass
}

func (n ingressNetworkingV1Beta1Strategy) ConvertIngress() *Ingress {
	return &Ingress{
		APIVersion:  NETWORKINGV1BETA1,
		Namespace:   n.ig.GetNamespace(),
		Name:        n.ig.GetName(),
		Class:       getIgClass(n.ig.Spec.IngressClassName),
		Annotations: CopyAnnotations(n.ig.GetAnnotations()),
		Rules: func(ingressRules []networkingv1beta1.IngressRule) map[string]*IngressRule {
			rules := make(map[string]*IngressRule)
			for _, k8sRule := range ingressRules {
				paths := make(map[string]*IngressPath)
				if k8sRule.HTTP == nil {
					logger.Warningf("Ingress HTTP rules for [%s] does not exists", k8sRule.Host)
					continue
				}
				for _, k8sPath := range k8sRule.HTTP.Paths {
					prefix := ""
					if k8sPath.PathType != nil {
						prefix = string(*k8sPath.PathType)
					}
					paths[prefix+"-"+k8sPath.Path] = &IngressPath{
						Path:          k8sPath.Path,
						PathTypeMatch: string(*k8sPath.PathType),
						SvcName:       k8sPath.Backend.ServiceName,
						SvcPortInt:    int64(k8sPath.Backend.ServicePort.IntValue()),
						SvcPortString: k8sPath.Backend.ServicePort.StrVal,
						Status:        "",
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
		}(n.ig.Spec.Rules),
		DefaultBackend: func(ingressBackend *networkingv1beta1.IngressBackend) *IngressPath {
			if ingressBackend == nil {
				return nil
			}
			return &IngressPath{
				SvcName:          ingressBackend.ServiceName,
				SvcPortInt:       int64(ingressBackend.ServicePort.IntValue()),
				SvcPortString:    ingressBackend.ServicePort.StrVal,
				IsDefaultBackend: true,
				Status:           "",
			}
		}(n.ig.Spec.Backend),
		TLS: func(ingressTLS []networkingv1beta1.IngressTLS) map[string]*IngressTLS {
			tls := make(map[string]*IngressTLS)
			for _, k8sTLS := range ingressTLS {
				for _, host := range k8sTLS.Hosts {
					tls[host] = &IngressTLS{
						Host:       host,
						SecretName: k8sTLS.SecretName,
						Status:     EMPTY,
					}
				}
			}
			return tls
		}(n.ig.Spec.TLS),
		Status: func() Status {
			if n.ig.ObjectMeta.GetDeletionTimestamp() != nil {
				return DELETED
			}
			return ADDED
		}(),
	}
}

func (n ingressNetworkingV1Beta1Strategy) ConvertClass() *IngressClass {
	return &IngressClass{
		APIVersion: NETWORKINGV1BETA1,
		Name:       n.class.GetName(),
		Controller: n.class.Spec.Controller,
		Status: func() Status {
			if n.class.ObjectMeta.GetDeletionTimestamp() != nil {
				return DELETED
			}
			return ADDED
		}(),
	}
}

// ingressExtensionsStrategy is the Strategy implementation for converting an
// ingresses.extensions/v1beta1 object into a store.Ingress resource.
type ingressExtensionsStrategy struct {
	ig *extensionsv1beta1.Ingress
}

func (e ingressExtensionsStrategy) ConvertIngress() *Ingress {
	return &Ingress{
		APIVersion:  EXTENSIONSV1BETA1,
		Namespace:   e.ig.GetNamespace(),
		Name:        e.ig.GetName(),
		Annotations: CopyAnnotations(e.ig.GetAnnotations()),
		Rules: func(ingressRules []extensionsv1beta1.IngressRule) map[string]*IngressRule {
			rules := make(map[string]*IngressRule)
			for _, k8sRule := range ingressRules {
				paths := make(map[string]*IngressPath)
				if k8sRule.HTTP == nil {
					logger.Warningf("Ingress HTTP rules for [%s] does not exists", k8sRule.Host)
					continue
				}
				for _, k8sPath := range k8sRule.HTTP.Paths {
					prefix := ""
					if k8sPath.PathType != nil {
						prefix = string(*k8sPath.PathType)
					}
					paths[prefix+"-"+k8sPath.Path] = &IngressPath{
						Path:          k8sPath.Path,
						PathTypeMatch: string(*k8sPath.PathType),
						SvcName:       k8sPath.Backend.ServiceName,
						SvcPortInt:    int64(k8sPath.Backend.ServicePort.IntValue()),
						SvcPortString: k8sPath.Backend.ServicePort.StrVal,
						Status:        "",
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
		}(e.ig.Spec.Rules),
		DefaultBackend: func(ingressBackend *extensionsv1beta1.IngressBackend) *IngressPath {
			if ingressBackend == nil {
				return nil
			}
			return &IngressPath{
				SvcName:          ingressBackend.ServiceName,
				SvcPortInt:       int64(ingressBackend.ServicePort.IntValue()),
				SvcPortString:    ingressBackend.ServicePort.StrVal,
				IsDefaultBackend: true,
				Status:           "",
			}
		}(e.ig.Spec.Backend),
		TLS: func(ingressTLS []extensionsv1beta1.IngressTLS) map[string]*IngressTLS {
			tls := make(map[string]*IngressTLS)
			for _, k8sTLS := range ingressTLS {
				for _, host := range k8sTLS.Hosts {
					tls[host] = &IngressTLS{
						Host:       host,
						SecretName: k8sTLS.SecretName,
						Status:     EMPTY,
					}
				}
			}
			return tls
		}(e.ig.Spec.TLS),
		Status: func() Status {
			if e.ig.ObjectMeta.GetDeletionTimestamp() != nil {
				return DELETED
			}
			return ADDED
		}(),
	}
}

// ingressNetworkingV1Strategy is the Strategy implementation for converting an
// ingresses.networking.k8s.io/v1 object into a store.Ingress resource.
type ingressNetworkingV1Strategy struct {
	ig    *networkingv1.Ingress
	class *networkingv1.IngressClass
}

func (n ingressNetworkingV1Strategy) ConvertIngress() *Ingress {
	return &Ingress{
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
					prefix := ""
					if k8sPath.PathType != nil {
						prefix = string(*k8sPath.PathType)
					}
					paths[prefix+"-"+k8sPath.Path] = &IngressPath{
						Path:          k8sPath.Path,
						PathTypeMatch: string(*k8sPath.PathType),
						SvcName:       k8sPath.Backend.Service.Name,
						SvcPortInt:    int64(k8sPath.Backend.Service.Port.Number),
						SvcPortString: k8sPath.Backend.Service.Port.Name,
						Status:        "",
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
		}(n.ig.Spec.Rules),
		DefaultBackend: func(ingressBackend *networkingv1.IngressBackend) *IngressPath {
			if ingressBackend == nil {
				return nil
			}
			return &IngressPath{
				SvcName:          ingressBackend.Service.Name,
				SvcPortInt:       int64(ingressBackend.Service.Port.Number),
				SvcPortString:    ingressBackend.Service.Port.Name,
				IsDefaultBackend: true,
				Status:           "",
			}
		}(n.ig.Spec.DefaultBackend),
		TLS: func(ingressTLS []networkingv1.IngressTLS) map[string]*IngressTLS {
			tls := make(map[string]*IngressTLS)
			for _, k8sTLS := range ingressTLS {
				for _, host := range k8sTLS.Hosts {
					tls[host] = &IngressTLS{
						Host:       host,
						SecretName: k8sTLS.SecretName,
						Status:     EMPTY,
					}
				}
			}
			return tls
		}(n.ig.Spec.TLS),
		Status: func() Status {
			if n.ig.ObjectMeta.GetDeletionTimestamp() != nil {
				return DELETED
			}
			return ADDED
		}(),
	}
}

func (n ingressNetworkingV1Strategy) ConvertClass() *IngressClass {
	return &IngressClass{
		APIVersion: NETWORKINGV1,
		Name:       n.class.GetName(),
		Controller: n.class.Spec.Controller,
		Status: func() Status {
			if n.class.ObjectMeta.GetDeletionTimestamp() != nil {
				return DELETED
			}
			return ADDED
		}(),
	}
}

func getIgClass(className *string) string {
	if className == nil {
		return ""
	}
	return *className
}
