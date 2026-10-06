package annotations

import (
	"slices"
	"strings"

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
		type originWithPriority struct {
			origin        string
			orderPriority int
		}
		var originsWithPriority []originWithPriority
		for origin, annotationData := range annotationDataByOrigin {
			originsWithPriority = append(originsWithPriority, originWithPriority{origin: origin, orderPriority: annotationData.orderPriority})
		}
		// Sort by orderPriority descending
		slices.SortFunc(originsWithPriority, func(a, b originWithPriority) int {
			if a.orderPriority == b.orderPriority {
				return strings.Compare(a.origin, b.origin)
			}
			return b.orderPriority - a.orderPriority
		})
		values := []string{}
		for _, origin := range originsWithPriority {
			annotationData := annotationDataByOrigin[origin.origin]
			instance.ReloadIf(annotationData.status != store.EMPTY,
				"custom annotations from %s has been created/modified/deleted", origin.origin)

			// The entry has not been reseen so delete it.
			if annotationData.status == store.DELETED {
				delete(customAnnotationState.backends[backend], origin.origin)
				continue
			}

			instance.ReloadIf(len(annotationData.updated) > 0,
				"custom annotations from %s has been updated", origin.origin)

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
