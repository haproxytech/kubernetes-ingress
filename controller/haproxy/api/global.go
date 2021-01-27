package api

import (
	"fmt"
	"reflect"
	"strings"

	parser "github.com/haproxytech/config-parser/v3"
	"github.com/haproxytech/config-parser/v3/types"
)

// section can be "global" or "defaults"
func (c *clientNative) GlobalConfigEnabled(section string, config string) (enabled bool, err error) {
	var pSection parser.Section
	switch section {
	case "global":
		pSection = parser.Global
	case "defaults":
		pSection = parser.Defaults
	default:
		return false, fmt.Errorf("incorrect section '%s': it can be either global or defaults", section)
	}
	p, pErr := c.getParser(pSection, "", config)
	if p == nil {
		return false, pErr
	} else if data, err := p.Get(false); data == nil {
		return false, err
	}
	return true, nil
}

// section can be "global" or "defaults"
func (c *clientNative) GlobalWriteConfig(section string, config string) (result string, err error) {
	var pSection parser.Section
	switch section {
	case "global":
		pSection = parser.Global
	case "defaults":
		pSection = parser.Defaults
	default:
		return "", fmt.Errorf("incorrect section '%s': it can be either global or defaults", section)
	}
	p, err := c.getParser(pSection, "", config)
	if p == nil {
		return "", err
	}
	lines, _, err := p.ResultAll()
	if err != nil {
		return "", err
	}
	buf := make([]string, 0, len(lines))
	for _, line := range lines {
		buf = append(buf, line.Data)
	}
	return strings.Join(buf, "\n"), nil
}

func (c *clientNative) GlobalCfgSnippet(value *types.StringSliceC) error {
	return c.setSectionAttribute(parser.Global, "config-snippet", value)
}

func (c *clientNative) DaemonMode(value *types.Enabled) error {
	return c.setSectionAttribute(parser.Global, "daemon", value)
}

func (c *clientNative) DefaultLogFormat(value *types.StringC) error {
	return c.setSectionAttribute(parser.Defaults, "log-format", value)
}

func (c *clientNative) GlobalMaxconn(value *types.Int64C) error {
	return c.setSectionAttribute(parser.Global, "maxconn", value)
}

func (c *clientNative) GlobalHardStopAfter(value *types.StringC) error {
	return c.setSectionAttribute(parser.Global, "hard-stop-after", value)
}

func (c *clientNative) DefaultOption(option string, value *types.SimpleOption) error {
	return c.setSectionAttribute(parser.Defaults, fmt.Sprintf("option %s", option), value)
}

func (c *clientNative) DefaultTimeout(timeout string, value *types.SimpleTimeout) error {
	return c.setSectionAttribute(parser.Defaults, fmt.Sprintf("timeout %s", timeout), value)
}

func (c *clientNative) DefaultErrorFile(value *types.ErrorFile, index int) error {
	return c.setSectionAttribute(parser.Defaults, "errorfile", value, index)
}

func (c *clientNative) LogTarget(value *types.Log, index int) error {
	return c.setSectionAttribute(parser.Global, "log", value, index)
}

func (c *clientNative) Nbthread(value *types.Int64C) error {
	return c.setSectionAttribute(parser.Global, "nbthread", value)
}

func (c *clientNative) PIDFile(value *types.StringC) error {
	return c.setSectionAttribute(parser.Global, "pidfile", value)
}

func (c *clientNative) RuntimeSocket(value *types.Socket) error {
	return c.setSectionAttribute(parser.Global, "socket", value)
}

func (c *clientNative) ServerStateBase(value *types.StringC) error {
	return c.setSectionAttribute(parser.Global, "server-state-base", value)
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
	// Set config value
	switch strings.Fields(attribute)[0] {
	case "config-snippet":
		value = value.(*types.StringSliceC)
	case "daemon":
		value = value.(*types.Enabled)
	case "log":
		value = value.(*types.Log)
	case "log-format", "pidfile", "server-state-base", "hard-stop-after":
		value = value.(*types.StringC)
	case "nbthread", "maxconn":
		value = value.(*types.Int64C)
	case "option":
		value = value.(*types.SimpleOption)
	case "timeout":
		value = value.(*types.SimpleTimeout)
	case "errorfile":
		value = value.(*types.ErrorFile)
	case "socket":
		attribute = "stats socket"
		value = value.(*types.Socket)
	default:
		return fmt.Errorf("unsupported attribute '%s'", attribute)
	}
	// Delete config
	if reflect.ValueOf(value).IsNil() {
		err = config.Set(section, sectionName, attribute, nil)
		if err == nil {
			c.activeTransactionHasChanges = true
		}
		return err
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
