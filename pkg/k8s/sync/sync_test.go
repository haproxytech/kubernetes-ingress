package k8ssync

import "testing"

func TestIsNamespacedSessionEvent(t *testing.T) {
	t.Parallel()
	namespaced := []SyncType{
		SERVICE, SECRET, INGRESS, ENDPOINTS,
		CR_GLOBAL, CR_DEFAULTS, CR_BACKEND, CR_FRONTEND, CR_TCP,
		GATEWAY, TCPROUTE, REFERENCEGRANT, PUBLISH_SERVICE, CUSTOM_RESOURCE,
	}
	for _, typ := range namespaced {
		if !IsNamespacedSessionEvent(typ) {
			t.Errorf("%s should be a namespaced session event", typ)
		}
	}
	for _, typ := range []SyncType{NAMESPACE, COMMAND, CONFIGMAP, POD, INGRESS_CLASS, GATEWAYCLASS, NAMESPACE_SESSION_READY} {
		if IsNamespacedSessionEvent(typ) {
			t.Errorf("%s must not be epoch-checked", typ)
		}
	}
}
