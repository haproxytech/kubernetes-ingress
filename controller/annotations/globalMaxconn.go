package annotations

import (
	"strconv"

	"github.com/haproxytech/client-native/v2/models"
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

func (a *GlobalMaxconn) Parse(input string) error {
	v, err := strconv.Atoi(input)
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
