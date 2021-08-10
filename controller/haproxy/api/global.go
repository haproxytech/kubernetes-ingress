package api

import (
	"github.com/haproxytech/client-native/v2/models"
	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/types"
)

func (c *clientNative) DefaultsGetConfiguration() (defaults *models.Defaults, err error) {
	_, defaults, err = c.nativeAPI.Configuration.GetDefaultsConfiguration(c.activeTransaction)
	return
}

func (c *clientNative) DefaultsPushConfiguration(defaults *models.Defaults) error {
	return c.nativeAPI.Configuration.PushDefaultsConfiguration(defaults, c.activeTransaction, 0)
}

func (c *clientNative) GlobalCfgSnippet(value []string) error {
	config, err := c.nativeAPI.Configuration.GetParser(c.activeTransaction)
	if err != nil {
		return err
	}
	if len(value) == 0 {
		err = config.Set(parser.Global, parser.GlobalSectionName, "config-snippet", nil)
	} else {
		err = config.Set(parser.Global, parser.GlobalSectionName, "config-snippet", types.StringSliceC{Value: value})
	}
	return err
}

func (c *clientNative) GlobalCreateLogTarget(logTarget *models.LogTarget) error {
	var err error
	c.activeTransactionHasChanges = true
	err = c.nativeAPI.Configuration.CreateLogTarget(string(parser.Global), parser.GlobalSectionName, logTarget, c.activeTransaction, 0)
	return err
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

func (c *clientNative) GlobalGetConfiguration() (global *models.Global, err error) {
	_, global, err = c.nativeAPI.Configuration.GetGlobalConfiguration(c.activeTransaction)
	return
}

func (c *clientNative) GlobalPushConfiguration(global *models.Global) error {
	return c.nativeAPI.Configuration.PushGlobalConfiguration(global, c.activeTransaction, 0)
}
