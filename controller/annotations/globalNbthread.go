package annotations

import (
	"runtime"
	"strconv"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type GlobalNbthread struct {
	name   string
	data   int64
	global *models.Global
}

func NewGlobalNbthread(n string, g *models.Global) *GlobalNbthread {
	return &GlobalNbthread{name: n, global: g}
}

func (a *GlobalNbthread) GetName() string {
	return a.name
}

func (a *GlobalNbthread) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	if input.Status == store.DELETED {
		return nil
	}
	v, err := strconv.Atoi(input.Value)
	if err != nil {
		return err
	}
	maxProcs := runtime.GOMAXPROCS(0)
	if v > maxProcs {
		v = maxProcs
	}
	a.data = int64(v)
	return nil
}

func (a *GlobalNbthread) Update() error {
	if a.data == 0 {
		logger.Infof("Removing nbThread option")
	} else {
		logger.Infof("Setting nbThread to %d", a.data)
	}
	a.global.Nbthread = a.data
	return nil
}
