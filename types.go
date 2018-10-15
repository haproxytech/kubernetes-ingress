package main

import (
	"errors"
	"log"

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
	Namespace   string
	IP          string
	Labels      map[string]string
	Status      v1.PodPhase
	Name        string
	HAProxyName string
	Maintenance bool //disabled
	Sorry       bool //backup
	Watch       watch.EventType
}

type Service struct {
	Name       string
	Namespace  string
	ClusterIP  string
	ExternalIP string
	Ports      []v1.ServicePort

	Annotations map[string]string
	Selector    map[string]string
	Watch       watch.EventType
}

type Namespace struct {
	Name        string
	Relevant    bool
	Annotations map[string]string
	Ingresses   map[string]*Ingress
	Pods        map[string]*Pod
	PodNames    map[string]bool
	Services    map[string]*Service
	Watch       watch.EventType
}

func (n *Namespace) GetServiceForPod(labels map[string]string) (*Service, error) {
	log.Println("GetServiceForPod", labels)
	for _, service := range n.Services {
		if hasSelectors(labels, service.Selector) {
			return service, nil
			log.Println("GetServiceForPod FOUND", labels, service)
		}
	}
	return nil, errors.New("service not found")
}

func (n *Namespace) GetPodsForSelector(selector map[string]string) map[string]*Pod {
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
	Watch       watch.EventType
}

type IngressRule struct {
	Host  string
	Paths map[string]*IngressPath
	Watch watch.EventType
}

type Ingress struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Rules       map[string]*IngressRule
	//Rules       []v1beta1.IngressRule
	Watch watch.EventType
}

type ConfigMap struct {
	Name  string
	Data  map[string]string
	Watch watch.EventType
}

type Secret struct {
	Name  string
	Data  map[string][]byte
	Watch watch.EventType
}
