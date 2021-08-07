package global

import (
	"runtime"
	"strconv"

	"github.com/haproxytech/client-native/v2/models"
)

type Nbthread struct {
	name   string
	global *models.Global
}

func NewNbthread(n string, g *models.Global) *Nbthread {
	return &Nbthread{name: n, global: g}
}

func (a *Nbthread) GetName() string {
	return a.name
}

func (a *Nbthread) Process(input string) error {
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
