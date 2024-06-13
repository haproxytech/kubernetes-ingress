package httprequests

import (
	"github.com/haproxytech/client-native/v5/models"
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

func Populate(client api.HAProxyClient, name string, httpRequests models.HTTPRequestRules, resource HTTP_REQUEST_DESTINATION) {
	currentHTTPRequests, errHTTPRequests := client.HTTPRequestRulesGet(string(resource), name)

	// There is a resource ...
	if errHTTPRequests == nil {
		diffHTTPRequests := httpRequests.Diff(currentHTTPRequests)
		// ... with different http requests from the resource.
		if len(diffHTTPRequests) != 0 {
			// ... we remove all the http requests
			_ = client.HTTPRequestRuleDeleteAll(string(resource), name)
			// ... and create all the new ones if any
			for _, httpRequest := range httpRequests {
				errCreate := client.HTTPRequestRuleCreate(string(resource), name, httpRequest)
				if errCreate != nil {
					utils.GetLogger().Err(errCreate)
					break
				}
			}
			// ... we reload because we created some http requests.
			instance.Reload("%s '%s', http requests updated: %+v", resource, name, diffHTTPRequests)
		}
	}
}
