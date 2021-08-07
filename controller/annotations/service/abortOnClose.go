package service

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type AbortOnClose struct {
	name    string
	backend *models.Backend
}

func NewAbortOnClose(n string, b *models.Backend) *AbortOnClose {
	return &AbortOnClose{name: n, backend: b}
}

func (a *AbortOnClose) GetName() string {
	return a.name
}

func (a *AbortOnClose) Process(input string) error {
	var enabled bool
	var err error
	if input != "" {
		enabled, err = utils.GetBoolValue(input, "abortonclose")
		if err != nil {
			return err
		}
	}
	if enabled {
		a.backend.Abortonclose = "enabled"
	} else {
		a.backend.Abortonclose = "disabled"
	}
	return nil
}
