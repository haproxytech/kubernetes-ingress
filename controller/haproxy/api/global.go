package api

import (
	"fmt"
	"strings"

	parser "github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/config-parser/v2/types"
)

var typeValue interface{}

func (c *clientNative) EnabledConfig(configType string) (enabled bool, err error) {
	// Get current Parser Instance
	if c.activeTransaction == "" {
		return false, fmt.Errorf("no active transaction")
	}
	config, err := c.nativeAPI.Configuration.GetParser(c.activeTransaction)
	if err != nil {
		return false, err
	}
	if configType == "daemon" {
		val, err := config.Get(parser.Global, parser.GlobalSectionName, "daemon")
		if val != nil {
			return true, err
		}
	}
	return false, fmt.Errorf("unsupported option '%s'", configType)
}

func (c *clientNative) SetDaemonMode(value *bool) error {
	typeValue = nil
	if value != nil {
		typeValue = *value
	}
	return c.setSectionAttribute(parser.Global, "daemon", typeValue)
}

func (c *clientNative) SetDefaulLogFormat(value *string) error {
	typeValue = nil
	if value != nil {
		typeValue = *value
	}
	return c.setSectionAttribute(parser.Defaults, "log-format", typeValue)
}

func (c *clientNative) SetDefaulMaxconn(value *int64) error {
	typeValue = nil
	if value != nil {
		typeValue = *value
	}
	return c.setSectionAttribute(parser.Global, "maxconn", typeValue)
}

func (c *clientNative) SetDefaulOption(option string, enabled *bool) error {
	typeValue = nil
	if enabled != nil {
		typeValue = *enabled
	}
	return c.setSectionAttribute(parser.Defaults, fmt.Sprintf("option %s", option), typeValue)
}

func (c *clientNative) SetDefaulTimeout(timeout string, value *string) error {
	typeValue = nil
	if value != nil {
		typeValue = *value
	}
	return c.setSectionAttribute(parser.Defaults, fmt.Sprintf("timeout %s", timeout), typeValue)
}

func (c *clientNative) SetLogTarget(log *types.Log, index int) error {
	typeValue = nil
	if log != nil {
		typeValue = *log
	}
	return c.setSectionAttribute(parser.Global, "log", typeValue, index)
}

func (c *clientNative) SetNbthread(value *int64) error {
	typeValue = nil
	if value != nil {
		typeValue = *value
	}
	return c.setSectionAttribute(parser.Global, "nbthread", typeValue)
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
	if value == nil {
		err = config.Set(section, sectionName, attribute, nil)
		if err == nil {
			c.activeTransactionHasChanges = true
		}
		return err
	}
	// Set config value
	var attributeValue interface{}
	switch strings.Fields(attribute)[0] {
	case "daemon":
		attributeValue = types.Enabled{}
	case "log":
		attributeValue = value.(types.Log)
	case "log-format":
		attributeValue = types.StringC{
			Value: "'" + value.(string) + "'",
		}
	case "nbthread", "maxconn":
		attributeValue = types.Int64C{
			Value: value.(int64),
		}
	case "option":
		attributeValue = types.SimpleOption{
			NoOption: !value.(bool),
		}
	case "timeout":
		attributeValue = types.SimpleTimeout{Value: value.(string)}
	default:
		return fmt.Errorf("insupported attribute '%s'", attribute)
	}
	if len(index) > 0 {
		err = config.Insert(section, sectionName, attribute, attributeValue, index[0])
	} else {
		err = config.Set(section, sectionName, attribute, attributeValue)
	}
	if err == nil {
		c.activeTransactionHasChanges = true
	}
	return err
}
