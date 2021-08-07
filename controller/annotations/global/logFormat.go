package global

import (
	"strings"

	"github.com/haproxytech/client-native/v2/models"
)

type LogFormat struct {
	name     string
	defaults *models.Defaults
}

func NewLogFormat(n string, d *models.Defaults) *LogFormat {
	return &LogFormat{name: n, defaults: d}
}

func (a *LogFormat) GetName() string {
	return a.name
}

func (a *LogFormat) Process(input string) error {
	if input != "" {
		input = "'" + strings.TrimSpace(input) + "'"
	}
	a.defaults.LogFormat = input
	return nil
}
