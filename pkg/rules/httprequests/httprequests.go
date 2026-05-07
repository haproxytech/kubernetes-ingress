package httprequests

import (
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func PopulateBackend(client api.HAProxyClient, name string, httpRequests models.HTTPRequestRules) {
	Populate(client, name, httpRequests, rules.ParentTypeBackend)
}

func PopulateFrontend(client api.HAProxyClient, name string, httpRequests models.HTTPRequestRules) {
	Populate(client, name, httpRequests, rules.ParentTypeFrontend)
}

func Populate(client api.HTTPRequestRule, name string, desired models.HTTPRequestRules, parentType rules.ParentType) {
	current, errGet := client.HTTPRequestRulesGet(string(parentType), name)
	if errGet != nil {
		utils.GetLogger().Err(errGet)
		return
	}
	diff := desired.Diff(current)
	if len(diff) == 0 {
		return
	}
	if err := client.HTTPRequestRulesReplace(string(parentType), name, desired); err != nil {
		utils.GetLogger().Err(err)
		return
	}
	instance.Reload("%s '%s', http request rules updated: %+v", parentType, name, diff)
}
