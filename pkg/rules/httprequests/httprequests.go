package httprequests

import (
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

//nolint:golint,stylecheck
type HTTP_REQUEST_DESTINATION string

//nolint:golint,stylecheck
const (
	HTTP_REQUEST_FRONTEND HTTP_REQUEST_DESTINATION = "frontend"
	HTTP_REQUEST_BACKEND  HTTP_REQUEST_DESTINATION = "backend"
)

func PopulateBackend(client api.HAProxyClient, name string, httpRequests models.HTTPRequestRules) {
	Populate(client, name, httpRequests, HTTP_REQUEST_BACKEND)
}

func PopulateFrontend(client api.HAProxyClient, name string, httpRequests models.HTTPRequestRules) {
	Populate(client, name, httpRequests, HTTP_REQUEST_FRONTEND)
}

func Populate(client api.HTTPRequestRule, name string, rules models.HTTPRequestRules, resource HTTP_REQUEST_DESTINATION) {
	currentHTTPRequests, errHTTPRequests := client.HTTPRequestRulesGet(string(resource), name)

	// There is a resource ...
	if errHTTPRequests == nil {
		diffHTTPRequests := rules.Diff(currentHTTPRequests)
		// ... with different http requests from the resource.
		if len(diffHTTPRequests) != 0 {
			err := client.HTTPRequestRulesReplace(string(resource), name, rules)
			if err != nil {
				utils.GetLogger().Err(err)
				return
			}
			// ... we reload because we created some http requests.
			instance.Reload("%s '%s', http requests updated: %+v", resource, name, diffHTTPRequests)
		}
	}
}
