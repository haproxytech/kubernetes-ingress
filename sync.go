package main

import (
	"k8s.io/apimachinery/pkg/watch"
)

type SyncType string

//SyncType values
const (
	COMMAND   SyncType = "COMMAND"
	INGRESS   SyncType = "INGRESS"
	NAMESPACE SyncType = "NAMESPACE"
	SERVICE   SyncType = "SERVICE"
	POD       SyncType = "POD"
	CONFIGMAP SyncType = "CONFIGMAP"
	SECRET    SyncType = "SECRET"
)

type SyncDataEvent struct {
	SyncType
	EventType watch.EventType
	Namespace *Namespace
	Service   *Service
	Pod       *Pod
	Ingress   *Ingress
	ConfigMap *ConfigMap
	Secret    *Secret
}
