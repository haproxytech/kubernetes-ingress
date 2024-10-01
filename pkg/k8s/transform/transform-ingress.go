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

package k8stransform

import (
	"sort"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"
)

func TransformIngress(obj interface{}) (interface{}, error) {
	ing, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return obj, nil
	}

	// Fields to remove
	ing.Labels = nil
	ing.ObjectMeta.ManagedFields = nil

	// Remove duplicates
	RemoveDuplicates(ing)

	return ing, nil
}

func RemoveDuplicates(ingress *networkingv1.Ingress) {
	tls1, containsDups1 := RemoveIngressTLSDuplicates(ingress.Spec.TLS)
	rules1, containsDups2 := RemoveIngressRuleDuplicates(ingress.Spec.Rules)
	logger := utils.GetK8sLogger()
	if containsDups1 || containsDups2 {
		// originalValue, _ := yaml.Marshal(ingress)
		logger.Warningf("[K8s duplicate] Ingress %s/%s contains dups. Removing them. Consider cleaning your ingress. Original value: %+v",
			ingress.Name,
			ingress.Namespace,
			ingress)
	}

	ingress.Spec.TLS = tls1
	ingress.Spec.Rules = rules1
}

func RemoveIngressRuleDuplicates(rules []networkingv1.IngressRule) ([]networkingv1.IngressRule, bool) {
	if rules == nil {
		return nil, false
	}
	containsDups := false
	var result []networkingv1.IngressRule
	// map key will be: <host>/<hash(rule)>
	seen := make(map[string]struct{})
	for _, rule := range rules {
		if _, ok := seen[rule.Host]; !ok {
			jRule, _ := yaml.Marshal(rule.IngressRuleValue)
			key := rule.Host + "/" + utils.Hash(jRule)
			if _, ok := seen[key]; ok {
				containsDups = true
				continue
			}
			seen[key] = struct{}{}
			result = append(result, rule)
		}
	}

	return result, containsDups
}

func RemoveIngressTLSDuplicates(tlsList []networkingv1.IngressTLS) ([]networkingv1.IngressTLS, bool) {
	if tlsList == nil {
		return nil, false
	}

	containsDups := false

	tlsWithoutHostDupls := make([]networkingv1.IngressTLS, 0)

	// First step, clean each tls.hosts by removing duplicates inside the hosts.
	for _, ingtls := range tlsList {
		// if max 1 host, nothing to do,no dups
		if len(ingtls.Hosts) <= 1 {
			tlsWithoutHostDupls = append(tlsWithoutHostDupls, ingtls)
			continue
		}
		// If multiple hosts, remove dups if there are some
		distinctHosts := map[string]struct{}{}
		for _, host := range ingtls.Hosts {
			distinctHosts[host] = struct{}{}
		}
		newHosts := make([]string, 0, len(distinctHosts))
		for host := range distinctHosts {
			newHosts = append(newHosts, host)
		}
		if len(newHosts) != len(ingtls.Hosts) {
			containsDups = true
		}
		sort.Strings(newHosts)
		ingtls.Hosts = newHosts
		tlsWithoutHostDupls = append(tlsWithoutHostDupls, ingtls)
	}

	result := make([]networkingv1.IngressTLS, 0)

	// Then remove duplicates among TLS themselves
	// map key will be: <secret>/<(hash(hosts))>
	distinctTLSMap := map[string]struct{}{}
	for _, ingtls := range tlsWithoutHostDupls {
		jTLS, _ := yaml.Marshal(ingtls)
		key := ingtls.SecretName + "/" + utils.Hash(jTLS)
		if _, ok := distinctTLSMap[key]; ok {
			containsDups = true
			continue
		}
		result = append(result, ingtls)
		distinctTLSMap[key] = struct{}{}
	}

	return result, containsDups
}
