package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"
	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/types"
)

func (c *clientNative) DefaultsGetConfiguration() (defaults *models.Defaults, err error) {
	_, defaults, err = c.nativeAPI.Configuration.GetDefaultsConfiguration(c.activeTransaction)
	if err != nil {
		return nil, fmt.Errorf("unable to get HAProxy's defaults section: %w", err)
	}
	return
}

func (c *clientNative) DefaultsPushConfiguration(defaults models.Defaults) (err error) {
	err = c.nativeAPI.Configuration.PushDefaultsConfiguration(&defaults, c.activeTransaction, 0)
	if err != nil {
		return fmt.Errorf("unable to update HAProxy's defaults section: %w", err)
	}
	return
}

func (c *clientNative) GlobalCfgSnippet(value []string) (err error) {
	var config parser.Parser
	config, err = c.nativeAPI.Configuration.GetParser(c.activeTransaction)
	if err != nil {
		return
	}
	if len(value) == 0 {
		err = config.Set(parser.Global, parser.GlobalSectionName, "config-snippet", nil)
	} else {
		err = config.Set(parser.Global, parser.GlobalSectionName, "config-snippet", types.StringSliceC{Value: value})
	}
	if err != nil {
		return fmt.Errorf("unable to update global config snippet: %w", err)
	}
	return
}

func (c *clientNative) GlobalGetLogTargets() (lg models.LogTargets, err error) {
	c.activeTransactionHasChanges = true
	_, lg, err = c.nativeAPI.Configuration.GetLogTargets("global", parser.GlobalSectionName, c.activeTransaction)
	if err != nil {
		return lg, fmt.Errorf("unable to get HAProxy's global log targets: %w", err)
	}
	return
}

func (c *clientNative) GlobalCreateLogTargets(logTargets models.LogTargets) error {
	var err error
	c.activeTransactionHasChanges = true
	for _, log := range logTargets {
		err = c.nativeAPI.Configuration.CreateLogTarget(string(parser.Global), parser.GlobalSectionName, log, c.activeTransaction, 0)
		if err != nil {
			return fmt.Errorf("unable to update HAProxy's global log targets: %w", err)
		}
	}
	return nil
}

func (c *clientNative) GlobalDeleteLogTargets() {
	c.activeTransactionHasChanges = true
	for {
		err := c.nativeAPI.Configuration.DeleteLogTarget(0, "global", parser.GlobalSectionName, c.activeTransaction, 0)
		if err != nil {
			break
		}
	}
}

func (c *clientNative) GlobalGetConfiguration() (*models.Global, error) {
	_, global, err := c.nativeAPI.Configuration.GetGlobalConfiguration(c.activeTransaction)
	if err != nil {
		return nil, fmt.Errorf("unable to get HAProxy's global section: %w", err)
	}
	return global, err
}

func (c *clientNative) GlobalPushConfiguration(global models.Global) (err error) {
	err = c.nativeAPI.Configuration.PushGlobalConfiguration(&global, c.activeTransaction, 0)
	if err != nil {
		return fmt.Errorf("unable to update HAProxy's global section: %w", err)
	}
	return
}
