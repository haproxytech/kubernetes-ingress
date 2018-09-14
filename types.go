package main

import (
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
)

type Pod struct {
	Namespace string
	IP        string
	//Port      string
	Labels map[string]string
	Status v1.PodPhase
	Name   string
}

type Service struct {
	Name       string
	Namespace  string
	ClusterIP  string
	ExternalIP string
	Ports      []v1.ServicePort

	Annotations map[string]string
	Selector    map[string]string
}

type Namespace struct {
	Name        string
	Watch       bool
	Annotations map[string]string
	Ingresses   map[string]*Ingress
	Pods        map[string]*Pod
	Services    map[string]*Service
}

type Ingress struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Rules       []v1beta1.IngressRule
}
