package annotations

import (
	"fmt"
	"strings"

	"github.com/go-test/deep"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/validators"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

//nolint:golint,stylecheck
const COMMENT_ENDING = " ###"

type CustomAnnotations struct {
	ingress  *store.Ingress
	service  *store.Service
	name     string
	frontend string
	backend  string
}

type annotationData struct {
	status        store.Status
	value         []string
	updated       []string
	orderPriority int
}

// customAnnotationState accumulates the rendered custom annotations.
// Client-native has no structured models for them, so the ingress
// controller renders and tracks them here.
//
// The code in this file need to be rewritten to avoid init,
// global variables and rather expose a clean interface.
var customAnnotationState struct {
	frontendsCustom map[string]map[string]*annotationData // [frontend][origin] = &annotationData{}
	backends        map[string]map[string]*annotationData // backends[backend][origin] = &annotationData{}
}

func init() { //nolint:gochecknoinits
	InitCustomAnnotations()
}

func InitCustomAnnotations() {
	customAnnotationState.frontendsCustom = make(map[string]map[string]*annotationData)
	customAnnotationState.backends = make(map[string]map[string]*annotationData)
}

// CleanCustomAnnotations removes deleted entries from the accumulated
// frontend custom annotations.
func CleanCustomAnnotations() {
	for _, v := range customAnnotationState.frontendsCustom {
		for annKey, annVal := range v {
			if annVal.status == store.DELETED {
				delete(v, annKey)
			}
		}
	}
}

type CustomAnnotationsOptions struct {
	Backend  *string
	Frontend *string
	Ingress  *store.Ingress
	Name     string
}

func NewCustomAnnotations(opts CustomAnnotationsOptions) *CustomAnnotations {
	frontend := ""
	backend := ""
	if opts.Backend != nil {
		backend = *opts.Backend
	}
	if opts.Frontend != nil {
		frontend = *opts.Frontend
	}
	return &CustomAnnotations{
		name:     opts.Name,
		frontend: frontend,
		backend:  backend,
		ingress:  opts.Ingress,
	}
}

func (a *CustomAnnotations) GetName() string {
	return a.name
}

func (a *CustomAnnotations) Process(k store.K8s, annotations ...map[string]string) error {
	switch {
	case a.frontend != "":
		validator, err := validators.Get()
		if err != nil {
			return fmt.Errorf("failed to get validator: %w", err)
		}
		customAnnotations := map[string]string{}
		for _, annotation := range annotations {
			for key, value := range annotation {
				if strings.HasPrefix(key, "frontend."+validator.Prefix()) {
					// First map providing an annotation wins: Service > Ingress > ConfigMap.
					name := strings.TrimPrefix(key, "frontend."+validator.Prefix())
					if _, ok := customAnnotations[name]; !ok {
						customAnnotations[name] = value
					}
				}
			}
		}
		return renderFrontendCustomAnnotations(customAnnotations, a, validator)
	case a.backend != "":
		validator, err := validators.Get()
		if err != nil {
			return fmt.Errorf("failed to get validator: %w", err)
		}
		customAnnotations := map[string]string{}
		for _, annotation := range annotations {
			for key, value := range annotation {
				if strings.HasPrefix(key, "backend."+validator.Prefix()) {
					// First map providing an annotation wins: Service > Ingress > ConfigMap.
					name := strings.TrimPrefix(key, "backend."+validator.Prefix())
					if _, ok := customAnnotations[name]; !ok {
						customAnnotations[name] = value
					}
				}
			}
		}
		renderBackendCustomAnnotations(customAnnotations, a, validator)
	}
	return nil
}

func UpdateFrontendCustomAnnotations(api api.HAProxyClient, frontends ...string) (updated []string, err error) {
	for _, ft := range frontends {
		customData, okCustom := customAnnotationState.frontendsCustom[ft]
		if !okCustom {
			continue
		}
		deleted := []string{}
		newData := make([]string, 0)
		for k, v := range customData {
			if v.status == store.DELETED {
				updated = append(updated, "DELETED: "+k)
				deleted = append(deleted, k)
				continue
			}
			newData = append(newData, v.value...)
			updated = append(updated, v.updated...)
		}

		if len(deleted) == 0 && len(newData) == 0 {
			continue
		}
		if len(newData) > 0 {
			newData = append(newData, "### custom annotations end ###")
		}
		err = api.FrontendCustomAnnotationsSet(ft, newData)
		if err != nil {
			return updated, err
		}

		for _, v := range customAnnotationState.frontendsCustom[ft] {
			v.updated = nil
		}
	}
	return updated, err
}

func (a *CustomAnnotations) SetService(service *store.Service) {
	a.service = service
}

func recordFrontendCustomAnnotation(frontend, origin string, data []string, orderPriority int) {
	var exists bool
	if _, exists = customAnnotationState.frontendsCustom[frontend][origin]; !exists {
		// Prevent an empty entry from being inserted (comment only)
		// and if no data is provided
		if len(data) == 1 || data == nil {
			return
		}
		customAnnotationState.frontendsCustom[frontend][origin] = &annotationData{status: store.ADDED, orderPriority: orderPriority}
	}

	currentData := customAnnotationState.frontendsCustom[frontend][origin]
	// As reseen it's not to be deleted
	if currentData.status == store.DELETED {
		currentData.status = store.EMPTY
	}

	updated := deep.Equal(currentData.value, data)
	// Something changed from the possibly existing entry ?
	// A new entry generates a difference between empty and something.
	if len(updated) != 0 {
		// A change so update.
		currentData.value = data
		currentData.updated = updated
		currentData.orderPriority = orderPriority
		if exists {
			// as existing, set status to modified.
			currentData.status = store.MODIFIED
		}
	}
}

func recordBackendCustomAnnotation(backend, origin string, data []string, orderPriority int) {
	var exists bool
	if _, exists = customAnnotationState.backends[backend][origin]; !exists {
		// Prevent an empty entry from being inserted (comment only)
		// and if no data is provided
		if len(data) == 1 || data == nil {
			return
		}
		customAnnotationState.backends[backend][origin] = &annotationData{status: store.ADDED, orderPriority: orderPriority}
	}

	currentData := customAnnotationState.backends[backend][origin]
	// As reseen it's not to be deleted
	if currentData.status == store.DELETED {
		currentData.status = store.EMPTY
	}

	updated := deep.Equal(currentData.value, data)
	// Something changed from the possibly existing entry ?
	// A new entry generates a difference between empty and something.
	if len(updated) != 0 {
		// A change so update.
		currentData.value = data
		currentData.updated = updated
		currentData.orderPriority = orderPriority
		if exists {
			// as existing, set status to modified.
			currentData.status = store.MODIFIED
		}
	}
}
