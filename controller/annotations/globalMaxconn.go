package annotations

import (
	"strconv"

	"github.com/haproxytech/config-parser/v2/types"

	"github.com/haproxytech/kubernetes-ingress/controller/configsnippet"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type globalMaxconn struct {
	data *types.Int64C
}

func (a *globalMaxconn) Overridden(configSnippet string) error {
	return configsnippet.NewGenericAttribute("maxconn").Overridden(configSnippet)
}

func (a *globalMaxconn) Parse(input string) error {
	v, err := strconv.Atoi(input)
	if err != nil {
		return err
	}
	a.data = &types.Int64C{Value: int64(v)}
	return nil
}

func (a *globalMaxconn) Delete(c api.HAProxyClient) Result {
	logger.Infof("Removing default maxconn")
	if err := c.GlobalMaxconn(nil); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}

func (a *globalMaxconn) Update(c api.HAProxyClient) Result {
	if a.data == nil {
		logger.Error("unable to update default maxconn: nil value")
		return NONE
	}
	logger.Infof("Setting default maxconn to: '%d'", a.data.Value)
	if err := c.GlobalMaxconn(a.data); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}
