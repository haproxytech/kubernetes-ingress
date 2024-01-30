package global

import (
	"strings"

	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type LogFormat struct {
	defaults *models.Defaults
	name     string
}

func NewLogFormat(n string, d *models.Defaults) *LogFormat {
	return &LogFormat{name: n, defaults: d}
}

func (a *LogFormat) GetName() string {
	return a.name
}

func (a *LogFormat) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input != "" {
		input = "'" + strings.TrimSpace(input) + "'"
	}
	a.defaults.LogFormat = input
	return nil
}
