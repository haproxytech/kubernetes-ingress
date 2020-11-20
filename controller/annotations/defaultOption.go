package annotations

import (
	"github.com/haproxytech/config-parser/v3/types"

	"github.com/haproxytech/kubernetes-ingress/controller/configsnippet"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type defaultOption struct {
	name string
	data *types.SimpleOption
}

func (a *defaultOption) Overridden(configSnippet string) error {
	return configsnippet.NewGenericAttribute("option " + a.name).Overridden(configSnippet)
}

func (a *defaultOption) Parse(input string) error {
	enabled, err := utils.GetBoolValue(input, a.name)
	if err != nil {
		return err
	}
	a.data = &types.SimpleOption{NoOption: !enabled}
	return nil
}

func (a *defaultOption) Delete(c api.HAProxyClient) Result {
	logger.Infof("Removing option %s", a.name)
	if err := c.DefaultOption(a.name, nil); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}

func (a *defaultOption) Update(c api.HAProxyClient) Result {
	if a.data.NoOption {
		logger.Infof("disabling %s", a.name)
	} else {
		logger.Infof("enabling %s", a.name)
	}
	if err := c.DefaultOption(a.name, a.data); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}
