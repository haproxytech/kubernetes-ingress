package service

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type Check struct {
	name   string
	server *models.Server
}

func NewCheck(n string, s *models.Server) *Check {
	return &Check{name: n, server: s}
}

func (a *Check) GetName() string {
	return a.name
}

func (a *Check) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		a.server.Check = ""
		return nil
	}
	enabled, err := utils.GetBoolValue(input, "check")
	if err != nil {
		return err
	}
	if enabled {
		a.server.Check = "enabled"
	} else {
		a.server.Check = "disabled"
	}
	return nil
}
