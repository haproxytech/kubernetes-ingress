package httpafterresponses

import (
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func PopulateBackend(client api.HAProxyClient, name string, desired models.HTTPAfterResponseRules) {
	Populate(client, name, desired, rules.ParentTypeBackend)
}

func PopulateFrontend(client api.HAProxyClient, name string, desired models.HTTPAfterResponseRules) {
	Populate(client, name, desired, rules.ParentTypeFrontend)
}

func Populate(client api.HTTPAfterResponseRule, name string, desired models.HTTPAfterResponseRules, parentType rules.ParentType) {
	current, errGet := client.HTTPAfterResponseRulesGet(string(parentType), name)
	if errGet != nil {
		utils.GetLogger().Err(errGet)
		return
	}
	diff := desired.Diff(current)
	if len(diff) == 0 {
		return
	}
	if err := client.HTTPAfterResponseRulesReplace(string(parentType), name, desired); err != nil {
		utils.GetLogger().Err(err)
		return
	}
	instance.Reload("%s '%s', http after response rules updated: %+v", parentType, name, diff)
}
