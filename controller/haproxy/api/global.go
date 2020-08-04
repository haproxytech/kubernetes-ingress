package api

import (
	"fmt"
	"reflect"
	"strings"

	parser "github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/config-parser/v2/types"
)

func (c *clientNative) GlobalConfigEnabled(config string) (enabled bool, err error) {
	p, pErr := c.getParser(parser.Global, "", config)
	if p == nil {
		return false, pErr
	}
	return true, nil
}

func (c *clientNative) SetGlobalCfgSnippet(value *types.StringSliceC) error {
	return c.setSectionAttribute(parser.Global, "config-snippet", value)
}

func (c *clientNative) SetDaemonMode(value *types.Enabled) error {
	return c.setSectionAttribute(parser.Global, "daemon", value)
}

func (c *clientNative) SetDefaultLogFormat(value *types.StringC) error {
	return c.setSectionAttribute(parser.Defaults, "log-format", value)
}

func (c *clientNative) SetGlobalMaxconn(value *types.Int64C) error {
	return c.setSectionAttribute(parser.Global, "maxconn", value)
}

func (c *clientNative) SetDefaultOption(option string, value *types.SimpleOption) error {
	return c.setSectionAttribute(parser.Defaults, fmt.Sprintf("option %s", option), value)
}

func (c *clientNative) SetDefaultTimeout(timeout string, value *types.SimpleTimeout) error {
	return c.setSectionAttribute(parser.Defaults, fmt.Sprintf("timeout %s", timeout), value)
}

func (c *clientNative) SetDefaultErrorFile(value *types.ErrorFile, index int) error {
	return c.setSectionAttribute(parser.Defaults, "errorfile", value, index)
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
		value = value.(*types.ErrorFile)
	default:
		return fmt.Errorf("unsupported attribute '%s'", attribute)
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

func (c *clientNative) getParser(section parser.Section, sectionName string, attribute string) (p parser.ParserInterface, err error) {
	if section == "" || attribute == "" {
		return nil, fmt.Errorf("missing param")
	}
	// Get current Parser Instance
	if c.activeTransaction == "" {
		return nil, fmt.Errorf("no active transaction")
	}
	config, err := c.nativeAPI.Configuration.GetParser(c.activeTransaction)
	if err != nil {
		return nil, err
	}
	// section lookup
	var parsers *parser.Parsers
	ok := false
	switch section {
	case parser.Global, parser.Defaults:
		parsers, ok = config.Parsers[section]["data"]
	case parser.Frontends, parser.Backends:
		parsers, ok = config.Parsers[section][sectionName]
	}
	if !ok {
		return nil, fmt.Errorf("section '%s %s' not found", section, sectionName)
	}
	// parser lookup
	for _, parser := range parsers.Parsers {
		if parser.GetParserName() == attribute {
			return parser, nil
		}
	}
	return nil, nil
}
