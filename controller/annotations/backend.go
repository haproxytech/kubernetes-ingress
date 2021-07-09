package annotations

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func HandleBackendAnnotations(backend *models.Backend, k8sStore store.K8s, client api.HAProxyClient, annotations ...map[string]string) {
	for _, a := range GetBackendAnnotations(client, backend) {
		annValue := k8sStore.GetValueFromAnnotations(a.GetName(), annotations...)
		if annValue == "" {
			continue
		}
		HandleAnnotation(a, annValue)
	}
}

func GetBackendAnnotations(client api.HAProxyClient, b *models.Backend) []Annotation {
	annotations := []Annotation{
		NewBackendCfgSnippet("backend-config-snippet", client, b),
		NewBackendAbortOnClose("abortonclose", b),
		NewBackendTimeoutCheck("timeout-check", b),
		NewBackendLoadBalance("load-balance", b),
		NewBackendCookie("cookie-persistence", b),
	}
	if b.Mode == "http" {
		annotations = append(annotations,
			NewBackendCheckHTTP("check-http", b),
			NewBackendForwardedFor("forwarded-for", b),
		)
	}
	return annotations
}
