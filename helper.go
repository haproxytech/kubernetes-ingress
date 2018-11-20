package main

import (
	"fmt"
	"log"
	"math/rand"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/watch"
)

//LogWatchEvent log what kind of event occured
func LogWatchEvent(t watch.EventType, watchType SyncType, ObjData ...interface{}) {
	if t == watch.Added {
		log.Println(watchType, "added", ObjData)
	}
	if t == watch.Deleted {
		log.Println(watchType, "deleted", ObjData)
	}
	if t == watch.Modified {
		log.Println(watchType, "modified", ObjData)
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

//ConvertLoadBalanceAlgorithm needed as lbctl has its own naming sistem
func ConvertLoadBalanceAlgorithm(algorithm string) (string, error) {
	//roundrobin least-connections hash-uri hash-source
	switch algorithm {
	case "roundrobin":
		return "roundrobin", nil
	case "leastconn":
		return "least-connections", nil
	case "uri":
		return "hash-uri", nil
	case "source":
		return "hash-source", nil
	default:
		return "roundrobin", fmt.Errorf("Balance algorithm %s not supported", algorithm)
	}
}
