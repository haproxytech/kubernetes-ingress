package annotations

import (
	"strings"

	"github.com/haproxytech/client-native/v2/models"
)

type DefaultLogFormat struct {
	name     string
	defaults *models.Defaults
}

func NewDefaultLogFormat(n string, d *models.Defaults) *DefaultLogFormat {
	return &DefaultLogFormat{name: n, defaults: d}
}

func (a *DefaultLogFormat) GetName() string {
	return a.name
}

func (a *DefaultLogFormat) Process(input string) error {
	if input != "" {
		input = "'" + strings.TrimSpace(input) + "'"
	}
	a.defaults.LogFormat = input
	return nil
}
