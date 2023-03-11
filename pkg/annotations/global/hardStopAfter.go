package global

import (
	"github.com/haproxytech/client-native/v3/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type HardStopAfter struct {
	global *models.Global
	name   string
}

func NewHardStopAfter(n string, g *models.Global) *HardStopAfter {
	return &HardStopAfter{name: n, global: g}
}

func (a *HardStopAfter) GetName() string {
	return a.name
}

func (a *HardStopAfter) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		a.global.HardStopAfter = nil
		return nil
	}
	v, err := utils.ParseTime(input)
	if err != nil {
		return err
	}
	a.global.HardStopAfter = v
	return nil
}
