package annotations

import (
	"runtime"
	"strconv"

	"github.com/haproxytech/config-parser/v3/types"

	"github.com/haproxytech/kubernetes-ingress/controller/configsnippet"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type nbthread struct {
	data *types.Int64C
}

func (a *nbthread) Overridden(configSnippet string) error {
	return configsnippet.NewGenericAttribute("nbthread").Overridden(configSnippet)
}

func (a *nbthread) Parse(input string) error {
	v, err := strconv.Atoi(input)
	if err != nil {
		return err
	}
	maxProcs := runtime.GOMAXPROCS(0)
	if v > maxProcs {
		v = maxProcs
	}
	a.data = &types.Int64C{Value: int64(v)}
	return nil
}

func (a *nbthread) Delete(c api.HAProxyClient) Result {
	logger.Infof("Removing nbThread option")
	if err := c.Nbthread(nil); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}

func (a *nbthread) Update(c api.HAProxyClient) Result {
	if a.data == nil {
		logger.Error("unable to update nbthread: nil value")
		return NONE
	}
	logger.Infof("Setting nbThread to: %d", a.data.Value)
	if err := c.Nbthread(a.data); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}
