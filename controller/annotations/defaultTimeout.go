package annotations

import (
	"strings"
	"time"

	"github.com/haproxytech/config-parser/v2/types"

	"github.com/haproxytech/kubernetes-ingress/controller/configsnippet"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type defaultTimeout struct {
	name string
	data *types.SimpleTimeout
}

func (a *defaultTimeout) Overridden(configSnippet string) error {
	return configsnippet.NewGenericAttribute("timeout " + a.name).Overridden(configSnippet)
}

func (a *defaultTimeout) Parse(input string) error {
	timeout, err := time.ParseDuration(input)
	if err != nil {
		return err
	}
	s := timeout.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	a.data = &types.SimpleTimeout{Value: s}
	return nil
}

func (a *defaultTimeout) Delete(c api.HAProxyClient) Result {
	logger.Infof("Removing default timeout-%s ", a.name)

	if err := c.DefaultTimeout(a.name, nil); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}

func (a *defaultTimeout) Update(c api.HAProxyClient) Result {
	logger.Infof("Setting default timeout-%s to %s", a.name, a.data.Value)
	if err := c.DefaultTimeout(a.name, a.data); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}
