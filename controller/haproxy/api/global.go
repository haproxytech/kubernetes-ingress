package api

import (
	"fmt"
	"reflect"
	"strings"

	parser "github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/config-parser/v2/types"
)

func (c *clientNative) GetConfig(configType string) (enabled bool, err error) {
	// Get current Parser Instance
	if c.activeTransaction == "" {
		return false, fmt.Errorf("no active transaction")
	}
	config, err := c.nativeAPI.Configuration.GetParser(c.activeTransaction)
	if err != nil {
		return false, err
	}
	if configType == "daemon" {
		_, err = config.Get(parser.Global, parser.GlobalSectionName, "daemon")
	} else {
		err = fmt.Errorf("unsupported config '%s'", configType)
	}
	if err == nil {
		return true, nil
	}
	if err.Error() == "no data" {
		return false, nil
	}
	return false, err
}

func (c *clientNative) SetConfigSnippet(value *types.StringSliceC) error {
	return c.setSectionAttribute(parser.Global, "config-snippet", value)
}

func (c *clientNative) SetDaemonMode(value *types.Enabled) error {
	return c.setSectionAttribute(parser.Global, "daemon", value)
}

func (c *clientNative) SetDefaultLogFormat(value *types.StringC) error {
	return c.setSectionAttribute(parser.Defaults, "log-format", value)
}

func (c *clientNative) SetDefaultMaxconn(value *types.Int64C) error {
	return c.setSectionAttribute(parser.Global, "maxconn", value)
}

func (c *clientNative) SetDefaultOption(option string, value *types.SimpleOption) error {
	return c.setSectionAttribute(parser.Defaults, fmt.Sprintf("option %s", option), value)
}

func (c *clientNative) ErrorFileDelete(index int) error {
	// Get current Parser Instance
	if c.activeTransaction == "" {
		return fmt.Errorf("no active transaction")
	}
	config, err := c.nativeAPI.Configuration.GetParser(c.activeTransaction)
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return config.Delete(parser.Defaults, parser.DefaultSectionName, "errorfile", index)
}

func (c *clientNative) ErrorFileCreate(code int, enabled *bool) error {
	if enabled == nil {
		return nil
	}
	typeValue := fmt.Sprintf("/etc/haproxy/errors/%d.http", code)
	return c.setSectionAttribute(parser.Defaults, fmt.Sprintf("errorfile %d", code), typeValue)
}

func (c *clientNative) SetDefaulTimeout(timeout string, value *types.SimpleTimeout) error {
	return c.setSectionAttribute(parser.Defaults, fmt.Sprintf("timeout %s", timeout), value)
}

func (c *clientNative) SetLogTarget(value *types.Log, index int) error {
	return c.setSectionAttribute(parser.Global, "log", value, index)
}

func (c *clientNative) SetNbthread(value *types.Int64C) error {
	return c.setSectionAttribute(parser.Global, "nbthread", value)
}

func (c *clientNative) setSectionAttribute(section parser.Section, attribute string, value interface{}, index ...int) error {
	// Get current Parser Instance
	if c.activeTransaction == "" {
		return fmt.Errorf("no active transaction")
	}
	config, err := c.nativeAPI.Configuration.GetParser(c.activeTransaction)
	if err != nil {
		return err
	}
	// Set HAProxy section name
	var sectionName string
	switch section {
	case parser.Defaults:
		sectionName = parser.DefaultSectionName
	case parser.Global:
		sectionName = parser.GlobalSectionName
	default:
		return fmt.Errorf("incorrect section type '%s'", section)
	}
	// Delete config
	if reflect.ValueOf(value).IsNil() {
		err = config.Set(section, sectionName, attribute, nil)
		if err == nil {
			c.activeTransactionHasChanges = true
		}
		return err
	}
	// Set config value
	switch strings.Fields(attribute)[0] {
	case "config-snippet":
		value = value.(*types.StringSliceC)
	case "daemon":
		value = value.(*types.Enabled)
	case "log":
		value = value.(*types.Log)
	case "log-format":
		value = value.(*types.StringC)
	case "nbthread", "maxconn":
		value = value.(*types.Int64C)
	case "option":
		value = value.(*types.SimpleOption)
	case "timeout":
		value = value.(*types.SimpleTimeout)
	case "errorfile":
		value = types.ErrorFile{
			Code: strings.Fields(attribute)[1],
			File: value.(string),
		}
		attribute = "errorfile"
	default:
		return fmt.Errorf("insupported attribute '%s'", attribute)
	}

	if len(index) > 0 {
		err = config.Insert(section, sectionName, attribute, value, index[0])
	} else {
		err = config.Set(section, sectionName, attribute, value)
	}
	if err == nil {
		c.activeTransactionHasChanges = true
	}
	return err
}
