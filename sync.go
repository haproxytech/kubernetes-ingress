package main

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
	Data interface{}
}
