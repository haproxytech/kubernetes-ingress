package annotations

import (
	"errors"
	"strings"

	"github.com/haproxytech/config-parser/v2/types"

	"github.com/haproxytech/kubernetes-ingress/controller/configsnippet"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type defaultLogFormat struct {
	data *types.StringC
}

func (a *defaultLogFormat) Overridden(configSnippet string) error {
	return configsnippet.NewGenericAttribute("log-format").Overridden(configSnippet)
}

func (a *defaultLogFormat) Parse(input string) error {
	input = strings.TrimSpace(input)
	if input == "" {
		return errors.New("unable to parse log-format: empty input")
	}
	a.data = &types.StringC{Value: "'" + input + "'"}
	return nil
}

func (a *defaultLogFormat) Delete(c api.HAProxyClient) Result {
	logger.Infof("Removing default log-format")
	if err := c.DefaultLogFormat(nil); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}

func (a *defaultLogFormat) Update(c api.HAProxyClient) Result {
	if a.data == nil {
		logger.Error("unable to update default log-format: nil value")
		return NONE
	}
	logger.Infof("Setting default log-format to: %s", a.data.Value)
	if err := c.DefaultLogFormat(a.data); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}
