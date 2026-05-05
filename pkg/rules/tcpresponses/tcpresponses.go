package tcpresponses

import (
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func PopulateBackend(client api.TCPResponseRule, name string, rules models.TCPResponseRules) {
	current, errGet := client.TCPResponseRulesGet("backend", name)
	if errGet != nil {
		return
	}
	diff := rules.Diff(current)
	if len(diff) == 0 {
		return
	}
	if err := client.TCPResponseRulesReplace("backend", name, rules); err != nil {
		utils.GetLogger().Err(err)
		return
	}
	instance.Reload("backend '%s', tcp response rules updated: %+v", name, diff)
}
