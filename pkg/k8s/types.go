package k8s

// SyncType represents type of k8s received message
type SyncType string

// k8s.SyncDataEvent represents converted k8s received message
type SyncDataEvent struct {
	_ [0]int
	SyncType
	Namespace string
	Name      string
	Data      interface{}
}

//nolint:golint,stylecheck
const (
	// SyncType values
	COMMAND       SyncType = "COMMAND"
	CONFIGMAP     SyncType = "CONFIGMAP"
	ENDPOINTS     SyncType = "ENDPOINTS"
	INGRESS       SyncType = "INGRESS"
	INGRESS_CLASS SyncType = "INGRESS_CLASS"
	NAMESPACE     SyncType = "NAMESPACE"
	POD           SyncType = "POD"
	SERVICE       SyncType = "SERVICE"
	SECRET        SyncType = "SECRET"
	CR_GLOBAL     SyncType = "Global"
	CR_DEFAULTS   SyncType = "Defaults"
	CR_BACKEND    SyncType = "Backend"
)
