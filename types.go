package main

import (
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

//Pod is usefull data from k8s structures about pod
type Pod struct {
	IP          string
	Labels      MapStringW
	Name        string
	HAProxyName string
	Maintenance bool //disabled
	Backends    map[string]struct{}
	Status      watch.EventType
}

//Service is usefull data from k8s structures about service
type Service struct {
	Name       string
	ClusterIP  string
	ExternalIP string
	Ports      []v1.ServicePort

	Annotations MapStringW
	Selector    MapStringW
	Status      watch.EventType
}

//Namespace is usefull data from k8s structures about namespace
type Namespace struct {
	_         [0]int
	Name      string
	Relevant  bool
	Ingresses map[string]*Ingress
	Pods      map[string]*Pod
	PodNames  map[string]bool
	Services  map[string]*Service
	Secret    map[string]*Secret
	Status    watch.EventType
}

//GetServicesForPod returns all services that are using this pod
func (n *Namespace) GetServicesForPod(labels MapStringW) ([]*Service, error) {
	result := []*Service{}
	for _, service := range n.Services {
		if hasSelectors(service.Selector, labels) {
			result = append(result, service)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("services not found for labels %s", labels.String())
	}
	return result, nil
}

//GetPodsForSelector returns all pod for defined selector
func (n *Namespace) GetPodsForSelector(selector MapStringW) map[string]*Pod {
	pods := make(map[string]*Pod)
	for _, pod := range n.Pods {
		if hasSelectors(selector, pod.Labels) {
			pods[pod.Name] = pod
		}
	}
	return pods
}

//IngressPath is usefull data from k8s structures about ingress path
type IngressPath struct {
	ServiceName string
	ServicePort int
	Path        string
	Status      watch.EventType
}

//IngressRule is usefull data from k8s structures about ingress rule
type IngressRule struct {
	Host   string
	Paths  map[string]*IngressPath
	Status watch.EventType
}

//Ingress is usefull data from k8s structures about ingress
type Ingress struct {
	Name        string
	Annotations MapStringW
	Rules       map[string]*IngressRule
	Status      watch.EventType
}

//ConfigMap is usefull data from k8s structures about configmap
type ConfigMap struct {
	Name        string
	Annotations MapStringW
	Status      watch.EventType
}

//Secret is usefull data from k8s structures about secret
type Secret struct {
	Name   string
	Data   map[string][]byte
	Status watch.EventType
}
