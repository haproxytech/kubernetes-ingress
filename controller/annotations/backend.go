package annotations

import (
	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func HandleBackendAnnotations(k8sStore store.K8s, client api.HAProxyClient, backend *models.Backend, forceParse bool, annotations ...store.MapStringW) (reload bool) {
	for _, a := range GetBackendAnnotations(client, backend) {
		annValue, _ := k8sStore.GetValueFromAnnotations(a.GetName(), annotations...)
		if annValue == nil {
			continue
		}
		reload = HandleAnnotation(a, *annValue, forceParse) || reload
	}
	return reload
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
