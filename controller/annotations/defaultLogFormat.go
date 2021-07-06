package annotations

import (
	"errors"
	"strings"

	"github.com/haproxytech/client-native/v2/models"
)

type DefaultLogFormat struct {
	name     string
	defaults *models.Defaults
	data     string
}

func NewDefaultLogFormat(n string, d *models.Defaults) *DefaultLogFormat {
	return &DefaultLogFormat{name: n, defaults: d}
}

func (a *DefaultLogFormat) GetName() string {
	return a.name
}

func (a *DefaultLogFormat) Parse(input string) error {
	a.data = strings.TrimSpace(input)
	if a.data == "" {
		return errors.New("unable to parse log-format: empty input")
	}
	return nil
}

func (a *DefaultLogFormat) Update() error {
	if a.data == "" {
		logger.Infof("Removing default log-format")
	} else {
		logger.Infof("Setting default log-format to: %s", a.data)
	}
	a.defaults.LogFormat = a.data
	return nil
}
