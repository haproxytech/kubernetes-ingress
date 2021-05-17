package annotations

import (
	"strconv"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type GlobalMaxconn struct {
	name   string
	data   int64
	global *models.Global
}

func NewGlobalMaxconn(n string, g *models.Global) *GlobalMaxconn {
	return &GlobalMaxconn{name: n, global: g}
}

func (a *GlobalMaxconn) GetName() string {
	return a.name
}

func (a *GlobalMaxconn) Parse(input store.StringW, forceParse bool) error {
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
	a.data = int64(v)
	return nil
}

func (a *GlobalMaxconn) Update() error {
	if a.data == 0 {
		logger.Infof("Removing global maxconn")
	} else {
		logger.Infof("Setting global maxconn to %d", a.data)
	}
	a.global.Maxconn = a.data
	return nil
}
