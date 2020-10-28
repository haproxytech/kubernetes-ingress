package annotations

import (
	"strings"
	"time"

	"github.com/haproxytech/config-parser/v3/types"
	"github.com/haproxytech/kubernetes-ingress/controller/configsnippet"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type globalHardStopAfter struct {
	data *types.StringC
}

func (ghsa *globalHardStopAfter) Overridden(configSnippet string) error {
	return configsnippet.NewGenericAttribute("hard-stop-after").Overridden(configSnippet)
}

func (ghsa *globalHardStopAfter) Parse(input string) error {
	after, err := time.ParseDuration(input)
	if err != nil {
		return err
	}
	duration := after.String()
	if strings.HasSuffix(duration, "m0s") {
		duration = duration[:len(duration)-2]
	}
	if strings.HasSuffix(duration, "h0m") {
		duration = duration[:len(duration)-2]
	}

	if err != nil {
		return err
	}

	ghsa.data = &types.StringC{Value: duration}
	return nil
}

func (ghsa *globalHardStopAfter) Delete(c api.HAProxyClient) Result {
	if err := c.GlobalHardStopAfter(nil); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}

func (ghsa *globalHardStopAfter) Update(c api.HAProxyClient) Result {
	if ghsa.data == nil {
		logger.Error("unable to update default hard-stop-after: nil value")
		return NONE
	}
	if err := c.GlobalHardStopAfter(ghsa.data); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}
