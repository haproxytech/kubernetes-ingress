package service

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type CheckInter struct {
	backend *models.Backend
	name    string
}

func NewCheckInter(n string, b *models.Backend) *CheckInter {
	return &CheckInter{name: n, backend: b}
}

func (a *CheckInter) GetName() string {
	return a.name
}

func (a *CheckInter) Process(k store.K8s, annotations ...map[string]string) error {
	if common.IsSet(a.name, annotations...) {
		utils.Once("deprecation-Backend-"+a.name, func() {
			logger.Warningf("DEPRECATED: annotation %q will be removed in a future version, use the Backend custom resource instead", a.name)
		})
	}
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		if a.backend.DefaultServer != nil {
			a.backend.DefaultServer.Inter = nil
		}
		return nil
	}
	value, err := utils.ParseTime(input)
	if err != nil {
		return err
	}
	if !common.ValidRule(common.BackendRule, fmt.Sprintf("default-server inter %d", *value)) {
		return fmt.Errorf("%s: HAProxy rejects the value '%s'", a.GetName(), input)
	}
	if a.backend.DefaultServer == nil {
		a.backend.DefaultServer = &models.DefaultServer{}
	}
	a.backend.DefaultServer.Inter = value
	return nil
}
