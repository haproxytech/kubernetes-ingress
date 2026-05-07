package httpresponses

import (
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func PopulateBackend(client api.HAProxyClient, name string, httpResponses models.HTTPResponseRules) {
	Populate(client, name, httpResponses, rules.ParentTypeBackend)
}

func PopulateFrontend(client api.HAProxyClient, name string, httpResponses models.HTTPResponseRules) {
	Populate(client, name, httpResponses, rules.ParentTypeFrontend)
}

func Populate(client api.HTTPResponseRule, name string, desired models.HTTPResponseRules, parentType rules.ParentType) {
	current, errGet := client.HTTPResponseRulesGet(string(parentType), name)
	if errGet != nil {
		utils.GetLogger().Err(errGet)
		return
	}
	diff := desired.Diff(current)
	if len(diff) == 0 {
		return
	}
	if err := client.HTTPResponseRulesReplace(string(parentType), name, desired); err != nil {
		utils.GetLogger().Err(err)
		return
	}
	instance.Reload("%s '%s', http response rules updated: %+v", parentType, name, diff)
}
