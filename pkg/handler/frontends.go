package handler

import (
	"dario.cat/mergo"
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type Frontend struct {
	crFrontendHTTP  *models.Frontend
	crFrontendHTTPS *models.Frontend
	crFrontendStats *models.Frontend
}

//nolint:golint, stylecheck
const (
	CUSTOM_RESOURCE_ANNOTATION_HTTP  = "cr-frontend-http"
	CUSTOM_RESOURCE_ANNOTATION_HTTPS = "cr-frontend-https"
	CUSTOM_RESOURCE_ANNOTATION_STATS = "cr-frontend-stats"
	FRONTEND_HTTP                    = "http"
	FRONTEND_HTTPS                   = "https"
	FRONTEND_STATS                   = "stats"
	FRONTEND_HEALTHZ                 = "healthz"
)

var mapFrontendAnnotations = map[string]string{
	CUSTOM_RESOURCE_ANNOTATION_HTTP:  FRONTEND_HTTP,
	CUSTOM_RESOURCE_ANNOTATION_HTTPS: FRONTEND_HTTPS,
	CUSTOM_RESOURCE_ANNOTATION_STATS: FRONTEND_STATS,
}

var managedFrontends = map[string]struct{}{
	FRONTEND_HTTP:  {},
	FRONTEND_HTTPS: {},
	FRONTEND_STATS: {},
	// FRONTEND_HEALTHZ: {},
}

// Update handles the creation, modification, or removal of the frontends based on the frontend custom resource
// It first gets the frontend from the custom resource, then gets the frontend to amend
// It then checks if the frontend from the custom resource and the frontend to amend are different
// If they are different, or if we switch from one frontend to another, it will first remove the old frontend
// Then it will create the new frontend if it doesn't already exist, otherwise it will update the existing frontend
func (handler *Frontend) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	errs := utils.Errors{}
	errs.Add(
		handler.manageFrontend(CUSTOM_RESOURCE_ANNOTATION_HTTP, k, h, &handler.crFrontendHTTP),
		handler.manageFrontend(CUSTOM_RESOURCE_ANNOTATION_HTTPS, k, h, &handler.crFrontendHTTPS),
		handler.manageFrontend(CUSTOM_RESOURCE_ANNOTATION_STATS, k, h, &handler.crFrontendStats),
		h.FrontendCfgSnippetApply(),
		h.FrontendDeletePending())

	frontends, _ := h.FrontendsGet()

	for _, frontend := range frontends {
		if _, managed := managedFrontends[frontend.FrontendBase.Name]; managed {
			continue
		}
		errCreate := h.FrontendCreateStructured(frontend)
		if errCreate != nil {
			errs.Add(h.FrontendEditStructured(frontend.FrontendBase.Name, frontend))
		}
	}

	return errs.Result()
}

// manageFrontend handles the creation, modification, or removal of a frontend based on the frontend custom resource
// It first gets the frontend from the custom resource, then gets the frontend to amend
// It then checks if the frontend from the custom resource and the frontend to amend are different
// If they are different, or if we switch from or to a frontend, it reloads the frontend
// Finally, it merges the frontend from the custom resource with the frontend to amend
// and creates or edits the merged frontend in the real configuration
func (handler *Frontend) manageFrontend(crFrontendAnnotationName string,
	k store.K8s, h haproxy.HAProxy,
	currentCR **models.Frontend,
) (err error) {
	// Get the frontend from the custom resource
	frontendCRFromAnnotation, _ := annotations.ModelFrontend(crFrontendAnnotationName, "", k, k.ConfigMaps.Main.Annotations)
	// Get the frontend to amend
	frontendName := mapFrontendAnnotations[crFrontendAnnotationName]
	frontend, err := h.FrontendGet(frontendName)
	if err != nil {
		return err
	}
	differentCRFromAnnotation := (*currentCR != nil && frontendCRFromAnnotation != nil && !frontendCRFromAnnotation.Equal(**currentCR)) // we have a CR and it is different
	switchToNone := (*currentCR != nil && frontendCRFromAnnotation == nil)                                                              // or now we switch to one
	switchToOne := (*currentCR == nil && frontendCRFromAnnotation != nil)                                                               // or now we switch to none
	needsReload := differentCRFromAnnotation || switchToNone || switchToOne
	msg := "Frontend '%s' updated by addition, modification or removal of frontend custom resource"
	switch {
	case differentCRFromAnnotation:
		msg = "Frontend '%s' updated by a different frontend custom resource"
	case switchToNone:
		msg = "Frontend '%s' updated by removal of frontend custom resource"
	case switchToOne:
		msg = "Frontend '%s' updated by addition of frontend custom resource"
	}
	instance.ReloadIf(needsReload, msg, frontendName)

	// Create a copy to not affect the original
	copyFrom := frontendCRFromAnnotation
	if copyFrom == nil {
		copyFrom = &models.Frontend{}
	}
	var frontendToBeMerged *models.Frontend
	frontendToBeMerged, err = copyFrontend(*copyFrom)
	if err != nil {
		return err
	}

	// Merge
	err = mergo.MergeWithOverwrite(frontendToBeMerged, frontend, mergo.WithAppendSlice)
	if err != nil {
		return err
	}
	// Keep track of the current state
	*currentCR = frontendCRFromAnnotation

	// Create/Edit the merged frontend in the real configuration
	err = h.FrontendCreateStructured(frontendToBeMerged)
	if err != nil {
		err = h.FrontendEditStructured(frontendToBeMerged.FrontendBase.Name, frontendToBeMerged)
	}
	return err
}

// copyFrontend returns a deep copy of the given frontend.
// It marshals the original frontend into a binary form,
// then unmarshals the binary form into a new frontend.
// If either the marshaling or unmarshaling fails, the function
// returns an error.
func copyFrontend(original models.Frontend) (*models.Frontend, error) {
	frontendCopy := &models.Frontend{}
	frontendToBeMergedContents, errMarshall := original.MarshalBinary()
	if errMarshall != nil {
		return nil, errMarshall
	}
	errUnmarshal := frontendCopy.UnmarshalBinary(frontendToBeMergedContents)
	return frontendCopy, errUnmarshal
}
