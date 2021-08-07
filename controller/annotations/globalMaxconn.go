package annotations

import (
	"strconv"

	"github.com/haproxytech/client-native/v2/models"
)

type GlobalMaxconn struct {
	name   string
	global *models.Global
}

func NewGlobalMaxconn(n string, g *models.Global) *GlobalMaxconn {
	return &GlobalMaxconn{name: n, global: g}
}

func (a *GlobalMaxconn) GetName() string {
	return a.name
}

func (a *GlobalMaxconn) Process(input string) error {
	if input == "" {
		a.global.Maxconn = 0
		return nil
	}
	v, err := strconv.Atoi(input)
	if err != nil {
		return err
	}
	a.global.Maxconn = int64(v)
	return nil
}
