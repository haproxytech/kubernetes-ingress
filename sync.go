package main

//SyncType represents type of k8s received message
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

//SyncDataEvent represents converted k8s received message
type SyncDataEvent struct {
	_ [0]int
	SyncType
	Namespace string
	Data      interface{}
}
