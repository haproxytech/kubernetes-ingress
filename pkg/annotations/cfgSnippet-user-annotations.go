package annotations

import (
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/validators"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func processCustomAnnotationsFrontend(customAnnotations map[string]string, a *CfgSnippet, validator *validators.Validator) error {
	haproxySection := "frontend"
	_, ok := cfgSnippet.frontendsCustom[a.frontend]
	if !ok {
		cfgSnippet.frontendsCustom[a.frontend] = map[string]*cfgData{}
	}
	if len(customAnnotations) == 0 {
		return nil
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
		customAnnotations, filterValues)

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

		// check if this is fine for snippet removal
		// comment := "### " + a.backend + ":" + origin + COMMENT_ENDING
		comment := "### " + origin + COMMENT_ENDING
		rdata := []string{comment}
		if validationErr != "" {
			rdata = append(rdata, validationErr)
		}
		if customAnnotationValue != "" {
			result := ""
			if prefix == "" {
				var err error
				result, err = validator.GetResult(k, customAnnotationValue, env)
				if err != nil {
					logger.Errorf("failed to get result for custom annotation '%s' for backend '%s': %s", k, a.backend, err.Error())
					rdata = append(rdata, "# ERROR: "+strings.ReplaceAll(err.Error(), "\n", "\n  # "))
				}
			}
			if strings.HasPrefix(result, "#") {
				rdata = append(rdata, result)
			} else {
				rdata = append(rdata, prefix+result)
			}
		}
		processConfigSnippetFrontendCustom(a.frontend, origin, rdata, len(keys)-index+1)
	}
	if len(keys) == 0 && len(cfgSnippet.frontendsCustom[a.frontend]) != 0 {
		// go through cfgSnippet.frontendsCustom[a.frontend] and mark them as deleted
		for _, value := range cfgSnippet.frontendsCustom[a.frontend] {
			value.status = store.DELETED
			value.value = []string{}
			value.updated = []string{}
		}
	}

	return nil
}

func processCustomAnnotationsBackend(customAnnotations map[string]string, a *CfgSnippet, validator *validators.Validator) {
	haproxySection := "backend"
	_, ok := cfgSnippet.backends[a.backend]
	if !ok {
		cfgSnippet.backends[a.backend] = map[string]*cfgData{}
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
		customAnnotations, filterValues)

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

		// check if this is fine for snippet removal
		// comment := "### " + a.backend + ":" + origin + COMMENT_ENDING
		comment := "### " + origin + COMMENT_ENDING
		rdata := []string{comment}
		if validationErr != "" {
			rdata = append(rdata, validationErr)
		}
		if customAnnotationValue != "" {
			result := ""
			if prefix == "" {
				var err error
				result, err = validator.GetResult(k, customAnnotationValue, env)
				if err != nil {
					logger.Errorf("failed to get result for custom annotation '%s' for backend '%s': %s", k, a.backend, err.Error())
					rdata = append(rdata, "# ERROR: "+strings.ReplaceAll(err.Error(), "\n", "\n  # "))
				}
			}
			if strings.HasPrefix(result, "#") {
				rdata = append(rdata, result)
			} else {
				rdata = append(rdata, prefix+result)
			}
		}
		processConfigSnippet(a.backend, origin, rdata, len(keys)-index+1)
	}
}
