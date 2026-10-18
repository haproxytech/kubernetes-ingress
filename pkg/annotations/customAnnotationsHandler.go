package annotations

import (
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type CustomAnnotationsHandler struct{}

func (h CustomAnnotationsHandler) Update(k store.K8s, api haproxy.HAProxy, ann Annotations) (err error) {
	return updateCustomAnnotations(api)
}

func updateCustomAnnotations(api api.HAProxyClient) (err error) {
	errs := utils.Errors{}
	// We iterate over each backend
	for backend, annotationDataByOrigin := range customAnnotationState.backends {
		// We must remove any previous insertion.
		err = api.BackendCustomAnnotationsSet(backend, nil)
		if err != nil {
			errs.Add(err)
			continue
		}
		// We need to sort origins by orderPriority
		values := []string{}
		for _, origin := range sortedOrigins(annotationDataByOrigin) {
			annotationData := annotationDataByOrigin[origin]
			instance.ReloadIf(annotationData.status != store.EMPTY,
				"custom annotations from %s has been created/modified/deleted", origin)

			// The entry has not been reseen so delete it.
			if annotationData.status == store.DELETED {
				delete(customAnnotationState.backends[backend], origin)
				continue
			}

			instance.ReloadIf(len(annotationData.updated) > 0,
				"custom annotations from %s has been updated", origin)

			values = append(values, annotationData.value...)
			annotationData.updated = nil
			// Mark the entry to be deleted if not reseen
			// Will be reset to EMPTY state if reseen in Process function.
			annotationData.status = store.DELETED
		}
		// Then insert it.
		err = api.BackendCustomAnnotationsSet(backend, values)
		if err != nil {
			errs.Add(err)
		}
		// Drop backends left with no entry
		if len(customAnnotationState.backends[backend]) == 0 {
			delete(customAnnotationState.backends, backend)
		}
	}
	return errs.Result()
}
