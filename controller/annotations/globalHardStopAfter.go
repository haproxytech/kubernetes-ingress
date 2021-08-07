package annotations

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type GlobalHardStopAfter struct {
	name   string
	global *models.Global
}

func NewGlobalHardStopAfter(n string, g *models.Global) *GlobalHardStopAfter {
	return &GlobalHardStopAfter{name: n, global: g}
}

func (a *GlobalHardStopAfter) GetName() string {
	return a.name
}

func (a *GlobalHardStopAfter) Process(input string) error {
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
