package annotations

import (
	"github.com/haproxytech/config-parser/v3/types"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type DefaultOption struct {
	name   string
	data   *types.SimpleOption
	client api.HAProxyClient
}

func NewDefaultOption(n string, c api.HAProxyClient) *DefaultOption {
	return &DefaultOption{
		name:   n,
		client: c,
	}
}

func (a *DefaultOption) GetName() string {
	return a.name
}

func (a *DefaultOption) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	if input.Status == store.DELETED {
		return nil
	}
	enabled, err := utils.GetBoolValue(input.Value, a.name)
	if err != nil {
		return err
	}
	a.data = &types.SimpleOption{NoOption: !enabled}
	return nil
}

func (a *DefaultOption) Update() error {
	if a.data == nil {
		logger.Infof("Removing option %s", a.name)
		return a.client.DefaultOption(a.name, nil)
	}
	if a.data.NoOption {
		logger.Infof("disabling option %s", a.name)
	} else {
		logger.Infof("enabling option %s", a.name)
	}
	return a.client.DefaultOption(a.name, a.data)
}
