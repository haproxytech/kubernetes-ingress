package main

import (
	"log"
	"math/rand"

	"k8s.io/api/extensions/v1beta1"
)

func LogErr(err error) {
	if err != nil {
		log.Println(err)
	}
}

func hasSelectors(selectors MapStringW, values MapStringW) bool {
	if len(selectors) == 0 {
		return false
	}
	for key, value1 := range selectors {
		value2, ok := values[key]
		if !ok {
			return false
		}
		if value1.Value != value2.Value {
			return false
		}
	}
	return true
}

var chars = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

//RandomString returns random string of size n
func RandomString(n int) string {
	b := make([]rune, n)
	size := len(chars)
	for i := range b {
		b[i] = chars[rand.Intn(size)]
	}
	return string(b)
}

//ConvertIngressRules converts data from kubernetes format
func ConvertIngressRules(ingressRules []v1beta1.IngressRule) map[string]*IngressRule {
	rules := make(map[string]*IngressRule)
	for _, k8sRule := range ingressRules {
		paths := make(map[string]*IngressPath)
		for _, k8sPath := range k8sRule.HTTP.Paths {
			paths[k8sPath.Path] = &IngressPath{
				Path:        k8sPath.Path,
				ServiceName: k8sPath.Backend.ServiceName,
				ServicePort: k8sPath.Backend.ServicePort.IntValue(),
				Status:      "",
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
