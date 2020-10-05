package annotations

import (
	"errors"
	"strings"

	"github.com/haproxytech/config-parser/v2/types"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type globalCfgSnippet struct {
	data *types.StringSliceC
}

// globalCfgSnippet cannot override itself
func (a *globalCfgSnippet) Overridden(configSnippet string) error {
	return nil
}

func (a *globalCfgSnippet) Parse(input string) error {
	var cfgLines []string
	for _, line := range strings.SplitN(strings.Trim(input, "\n"), "\n", -1) {
		if line = strings.TrimSpace(line); line != "" {
			cfgLines = append(cfgLines, line)
		}
	}
	if len(cfgLines) == 0 {
		return errors.New("unable to parse config-snippet: empty input")
	}
	a.data = &types.StringSliceC{Value: cfgLines}
	return nil
}

func (a *globalCfgSnippet) Delete(c api.HAProxyClient) Result {
	logger.Infof("Removing global config-snippet")
	if err := c.SetGlobalCfgSnippet(nil); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}

func (a *globalCfgSnippet) Update(c api.HAProxyClient) Result {
	if a.data == nil {
		logger.Error("unable to update global config-snippet: nil value")
		return NONE
	}
	logger.Infof("Setting global config-snippet to: %s", a.data.Value)
	if err := c.SetGlobalCfgSnippet(a.data); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}
