package service

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type Check struct {
	name    string
	backend *models.Backend
}

func NewCheck(n string, b *models.Backend) *Check {
	return &Check{name: n, backend: b}
}

func (a *Check) GetName() string {
	return a.name
}

func (a *Check) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		a.backend.DefaultServer.Check = ""
		return nil
	}
	enabled, err := utils.GetBoolValue(input, "check")
	if err != nil {
		return err
	}
	if enabled {
		a.backend.DefaultServer.Check = "enabled"
	} else {
		a.backend.DefaultServer.Check = "disabled"
	}
	return nil
}
