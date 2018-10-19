package main

import (
	"errors"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

/*//StructStatus state of the struck in any given moment
//mostly used to distinguish processed ones from one that are OK
type StructStatus string

//StructStatus possible states
const (
	StructStatusNormal   StructStatus = "NORMAL"
	StructStatusAdded    StructStatus = "ADDED"
	StructStatusModified StructStatus = "MODIFIED"
	StructStatusDeleted  StructStatus = "DELETED"
)*/

type Pod struct {
	IP          string
	Labels      MapStringW
	PodPhase    v1.PodPhase
	Name        string
	HAProxyName string
	Maintenance bool //disabled
	Sorry       bool //backup
	Status      watch.EventType
}

type Service struct {
	Name       string
	ClusterIP  string
	ExternalIP string
	Ports      []v1.ServicePort

	Annotations MapStringW
	Selector    MapStringW
	Status      watch.EventType
}

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

func (n *Namespace) GetServiceForPod(labels MapStringW) (*Service, error) {
	for _, service := range n.Services {
		if hasSelectors(labels, service.Selector) {
			return service, nil
		}
	}
	return nil, errors.New("service not found")
}

func (n *Namespace) GetPodsForSelector(selector MapStringW) map[string]*Pod {
	pods := make(map[string]*Pod)
	for _, pod := range n.Pods {
		if hasSelectors(selector, pod.Labels) {
			pods[pod.Name] = pod
		}
	}
	return pods
}

type IngressPath struct {
	ServiceName string
	ServicePort int
	Path        string
	Status      watch.EventType
}

type IngressRule struct {
	Host   string
	Paths  map[string]*IngressPath
	Status watch.EventType
}

type Ingress struct {
	Name        string
	Annotations MapStringW
	Rules       map[string]*IngressRule
	//Rules       []v1beta1.IngressRule
	Status watch.EventType
}

type ConfigMap struct {
	Name        string
	Annotations MapStringW
	Status      watch.EventType
}

type Secret struct {
	Name   string
	Data   map[string][]byte
	Status watch.EventType
}
