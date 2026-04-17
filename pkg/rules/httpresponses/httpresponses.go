package httpresponses

import (
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

//nolint:golint,stylecheck
type HTTP_RESPONSE_DESTINATION string

//nolint:golint,stylecheck
const (
	HTTP_RESPONSE_FRONTEND HTTP_RESPONSE_DESTINATION = "frontend"
	HTTP_RESPONSE_BACKEND  HTTP_RESPONSE_DESTINATION = "backend"
)

func PopulateBackend(client api.HAProxyClient, name string, httpResponses models.HTTPResponseRules) {
	Populate(client, name, httpResponses, HTTP_RESPONSE_BACKEND)
}

func PopulateFrontend(client api.HAProxyClient, name string, httpResponses models.HTTPResponseRules) {
	Populate(client, name, httpResponses, HTTP_RESPONSE_FRONTEND)
}

func Populate(client api.HTTPResponseRule, name string, rules models.HTTPResponseRules, resource HTTP_RESPONSE_DESTINATION) {
	currentHTTPResponses, errHTTPResponses := client.HTTPResponseRulesGet(string(resource), name)

	// There is a resource ...
	if errHTTPResponses == nil {
		diffHTTPResponses := rules.Diff(currentHTTPResponses)
		// ... with different http responses from the resource.
		if len(diffHTTPResponses) != 0 {
			err := client.HTTPResponseRulesReplace(string(resource), name, rules)
			if err != nil {
				utils.GetLogger().Err(err)
				return
			}
			// ... we reload because we created some http responses.
			instance.Reload("%s '%s', http responses updated: %+v", resource, name, diffHTTPResponses)
		}
	}
}
