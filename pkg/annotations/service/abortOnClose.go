package service

import (
	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type AbortOnClose struct {
	backend *models.Backend
	name    string
}

func NewAbortOnClose(n string, b *models.Backend) *AbortOnClose {
	return &AbortOnClose{name: n, backend: b}
}

func (a *AbortOnClose) GetName() string {
	return a.name
}

func (a *AbortOnClose) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
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
