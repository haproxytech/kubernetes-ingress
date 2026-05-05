package httpchecks

import (
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func PopulateBackend(client api.HTTPCheck, name string, rules models.HTTPChecks) {
	current, errGet := client.HTTPChecksGet("backend", name)
	if errGet != nil {
		return
	}
	diff := rules.Diff(current)
	if len(diff) == 0 {
		return
	}
	if err := client.HTTPChecksReplace("backend", name, rules); err != nil {
		utils.GetLogger().Err(err)
		return
	}
	instance.Reload("backend '%s', http checks updated: %+v", name, diff)
}
