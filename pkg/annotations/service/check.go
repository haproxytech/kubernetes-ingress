package service

import (
	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type Check struct {
	backend *models.Backend
	name    string
}

func NewCheck(n string, b *models.Backend) *Check {
	return &Check{name: n, backend: b}
}

func (a *Check) GetName() string {
	return a.name
}

// the value models.DefaultSever.Check should be a bool value and not an enum [enabled, disabled]
// this avoids an uncessary update when models.DefaultSever.Check is set form empty to "disabled"
func (a *Check) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		if a.backend.DefaultServer != nil {
			a.backend.DefaultServer.Check = ""
		}
		return nil
	}
	enabled, err := utils.GetBoolValue(input, "check")
	if err != nil {
		return err
	}
	if !enabled {
		if a.backend.DefaultServer != nil {
			a.backend.DefaultServer.Check = ""
		}
		return nil
	}
	if a.backend.DefaultServer == nil {
		a.backend.DefaultServer = &models.DefaultServer{ServerParams: models.ServerParams{Check: "enabled"}}
	} else {
		a.backend.DefaultServer.Check = "enabled"
	}
	return nil
}
