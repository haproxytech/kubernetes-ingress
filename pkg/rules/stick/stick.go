package stick

import (
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func PopulateBackend(client api.StickRule, name string, rules models.StickRules) {
	current, errGet := client.StickRulesGet(name)
	if errGet != nil {
		utils.GetLogger().Err(errGet)
		return
	}
	diff := rules.Diff(current)
	if len(diff) == 0 {
		return
	}
	if err := client.StickRulesReplace(name, rules); err != nil {
		utils.GetLogger().Err(err)
		return
	}
	instance.Reload("backend '%s', stick rules updated: %+v", name, diff)
}
