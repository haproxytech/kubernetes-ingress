package annotations

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type GlobalHardStopAfter struct {
	name   string
	data   *int64
	global *models.Global
}

func NewGlobalHardStopAfter(n string, g *models.Global) *GlobalHardStopAfter {
	return &GlobalHardStopAfter{name: n, global: g}
}

func (a *GlobalHardStopAfter) GetName() string {
	return a.name
}

func (a *GlobalHardStopAfter) Parse(input string) error {
	var err error
	a.data, err = utils.ParseTime(input)
	if err != nil {
		return err
	}
	return nil
}

func (a *GlobalHardStopAfter) Update() error {
	if a.data == nil {
		logger.Infof("Removing hard-stop-after timeout")
	} else {
		logger.Infof("Setting hard-stop-after to %ds", *a.data)
	}
	a.global.HardStopAfter = a.data
	return nil
}
