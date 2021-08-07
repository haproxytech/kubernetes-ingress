package global

import (
	"strconv"

	"github.com/haproxytech/client-native/v2/models"
)

type Maxconn struct {
	name   string
	global *models.Global
}

func NewMaxconn(n string, g *models.Global) *Maxconn {
	return &Maxconn{name: n, global: g}
}

func (a *Maxconn) GetName() string {
	return a.name
}

func (a *Maxconn) Process(input string) error {
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
