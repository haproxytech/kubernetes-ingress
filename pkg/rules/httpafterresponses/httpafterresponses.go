package httpafterresponses

import (
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

//nolint:golint,stylecheck
type HTTP_AFTER_RESPONSE_DESTINATION string

//nolint:golint,stylecheck
const (
	HTTP_AFTER_RESPONSE_FRONTEND HTTP_AFTER_RESPONSE_DESTINATION = "frontend"
	HTTP_AFTER_RESPONSE_BACKEND  HTTP_AFTER_RESPONSE_DESTINATION = "backend"
)

func PopulateBackend(client api.HAProxyClient, name string, rules models.HTTPAfterResponseRules) {
	Populate(client, name, rules, HTTP_AFTER_RESPONSE_BACKEND)
}

func PopulateFrontend(client api.HAProxyClient, name string, rules models.HTTPAfterResponseRules) {
	Populate(client, name, rules, HTTP_AFTER_RESPONSE_FRONTEND)
}

func Populate(client api.HTTPAfterResponseRule, name string, rules models.HTTPAfterResponseRules, resource HTTP_AFTER_RESPONSE_DESTINATION) {
	current, errGet := client.HTTPAfterResponseRulesGet(string(resource), name)

	if errGet == nil {
		diff := rules.Diff(current)
		if len(diff) != 0 {
			err := client.HTTPAfterResponseRulesReplace(string(resource), name, rules)
			if err != nil {
				utils.GetLogger().Err(err)
				return
			}
			instance.Reload("%s '%s', http after responses updated: %+v", resource, name, diff)
		}
	}
}
