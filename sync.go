package main

import (
	"k8s.io/apimachinery/pkg/watch"
)

//type SyncType int8
type SyncType string

//SyncType values
const (
	/*COMMAND   SyncType = 0
	INGRESS   SyncType = 1
	NAMESPACE SyncType = 2
	SERVICE   SyncType = 3
	POD       SyncType = 4
	CONFIGMAP SyncType = 5*/
	COMMAND   SyncType = "COMMAND"
	INGRESS   SyncType = "INGRESS"
	NAMESPACE SyncType = "NAMESPACE"
	SERVICE   SyncType = "SERVICE"
	POD       SyncType = "POD"
	CONFIGMAP SyncType = "CONFIGMAP"
)

type SyncDataEvent struct {
	SyncType
	EventType watch.EventType
	Namespace *Namespace
	Service   *Service
	Pod       *Pod
	Ingress   *Ingress
	ConfigMap *ConfigMap
}
