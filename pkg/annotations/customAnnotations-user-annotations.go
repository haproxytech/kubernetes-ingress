package annotations

import (
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/validators"
)

func renderFrontendCustomAnnotations(customAnnotations map[string]string, a *CustomAnnotations, validator *validators.Validator) error {
	haproxySection := "frontend"
	_, ok := customAnnotationState.frontendsCustom[a.frontend]
	if !ok {
		customAnnotationState.frontendsCustom[a.frontend] = map[string]*annotationData{}
	}
	// We have custom annotations, so we process them. in alphabetical order
	// to avoid issues with the order of processing.
	filterValues := validators.FilterValues{
		Section:  haproxySection,
		Frontend: a.frontend,
	}

	env := map[string]any{
		"FRONTEND": a.frontend,
	}
	keys := validator.GetSortedAnnotationKeys(
		customAnnotations, filterValues,
	)

	seen := make(map[string]struct{}, len(keys))
	for index, k := range keys {
		customAnnotationValue := customAnnotations[k]
		// If the custom annotation value is empty, we skip it.
		if customAnnotationValue == "" {
			continue
		}
		origin := validator.Prefix() + k
		prefix := ""
		// Validate the custom annotation
		var validationErr string
		if err := validator.ValidateInput(k, customAnnotationValue, haproxySection); err != nil {
			logger.Errorf("failed to validate custom annotation '%s' for %s '%s': %s", k, haproxySection, a.backend, err.Error())
			// continue
			validationErr = "# ERROR: " + strings.ReplaceAll(err.Error(), "\n", "\n  # ")
			customAnnotationValue = ""
			prefix = "# value:"
		}

		comment := "### " + origin + COMMENT_ENDING
		rdata := []string{comment}
		if validationErr != "" {
			rdata = append(rdata, validationErr)
		}
		if customAnnotationValue != "" {
			result := ""
			var err error
			if prefix == "" {
				result, err = validator.GetResult(k, customAnnotationValue, env)
				if err != nil {
					logger.Errorf("failed to get result for custom annotation '%s' for backend '%s': %s", k, a.backend, err.Error())
					rdata = append(rdata, "# ERROR: "+strings.ReplaceAll(err.Error(), "\n", "\n  # "))
				}
			}
			// A template may legitimately render nothing: treat that like an empty value.
			if err == nil && result == "" {
				continue
			}
			if strings.HasPrefix(result, "#") {
				rdata = append(rdata, result)
			} else {
				rdata = append(rdata, prefix+result)
			}
		}
		seen[origin] = struct{}{}
		recordFrontendCustomAnnotation(a.frontend, origin, rdata, len(keys)-index+1)
	}
	// Annotations that are no longer rendered must not keep their lines.
	dropUnseen(customAnnotationState.frontendsCustom[a.frontend], seen)

	return nil
}

func renderBackendCustomAnnotations(customAnnotations map[string]string, a *CustomAnnotations, validator *validators.Validator) {
	haproxySection := "backend"
	_, ok := customAnnotationState.backends[a.backend]
	if !ok {
		customAnnotationState.backends[a.backend] = map[string]*annotationData{}
	}
	if len(customAnnotations) == 0 {
		return
	}
	// We have custom annotations, so we process them. in alphabetical order
	// to avoid issues with the order of processing.
	filterValues := validators.FilterValues{
		Section:  haproxySection,
		Frontend: a.frontend,
		Backend:  a.backend,
	}

	env := map[string]any{
		"BACKEND":  a.backend,
		"FRONTEND": a.frontend,
	}
	if a.ingress != nil {
		env["NAMESPACE"] = a.ingress.Namespace
		env["INGRESS"] = a.ingress.Name
		filterValues.Namespace = a.ingress.Namespace
		filterValues.Ingress = a.ingress.Name
	}
	if a.service != nil {
		env["SERVICE"] = a.service.Name
		filterValues.Service = a.service.Name
	}
	keys := validator.GetSortedAnnotationKeys(
		customAnnotations, filterValues,
	)

	for index, k := range keys {
		customAnnotationValue := customAnnotations[k]
		// If the custom annotation value is empty, we skip it.
		if customAnnotationValue == "" {
			continue
		}
		origin := validator.Prefix() + k
		prefix := ""
		// Validate the custom annotation
		var validationErr string
		if err := validator.ValidateInput(k, customAnnotationValue, haproxySection); err != nil {
			logger.Errorf("failed to validate custom annotation '%s' for %s '%s': %s", k, haproxySection, a.backend, err.Error())
			// continue
			validationErr = "# ERROR: " + strings.ReplaceAll(err.Error(), "\n", "\n  # ")
			customAnnotationValue = ""
			prefix = "# value:"
		}

		comment := "### " + origin + COMMENT_ENDING
		rdata := []string{comment}
		if validationErr != "" {
			rdata = append(rdata, validationErr)
		}
		if customAnnotationValue != "" {
			result := ""
			var err error
			if prefix == "" {
				result, err = validator.GetResult(k, customAnnotationValue, env)
				if err != nil {
					logger.Errorf("failed to get result for custom annotation '%s' for backend '%s': %s", k, a.backend, err.Error())
					rdata = append(rdata, "# ERROR: "+strings.ReplaceAll(err.Error(), "\n", "\n  # "))
				}
			}
			// A template may legitimately render nothing: treat that like an empty value.
			if err == nil && result == "" {
				continue
			}
			if strings.HasPrefix(result, "#") {
				rdata = append(rdata, result)
			} else {
				rdata = append(rdata, prefix+result)
			}
		}
		recordBackendCustomAnnotation(a.backend, origin, rdata, len(keys)-index+1)
	}
}
