package annotations

import (
	"runtime"
	"strconv"

	"github.com/haproxytech/client-native/v2/models"
)

type GlobalNbthread struct {
	name   string
	global *models.Global
}

func NewGlobalNbthread(n string, g *models.Global) *GlobalNbthread {
	return &GlobalNbthread{name: n, global: g}
}

func (a *GlobalNbthread) GetName() string {
	return a.name
}

func (a *GlobalNbthread) Process(input string) error {
	if input == "" {
		a.global.Nbthread = 0
		return nil
	}

	v, err := strconv.Atoi(input)
	if err != nil {
		return err
	}
	maxProcs := runtime.GOMAXPROCS(0)
	if v > maxProcs {
		v = maxProcs
	}
	a.global.Nbthread = int64(v)
	return nil
}
