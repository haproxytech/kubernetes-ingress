package global

import (
	"strconv"

	"github.com/haproxytech/client-native/v3/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type Maxconn struct {
	global *models.Global
	name   string
}

func NewMaxconn(n string, g *models.Global) *Maxconn {
	return &Maxconn{name: n, global: g}
}

func (a *Maxconn) GetName() string {
	return a.name
}

func (a *Maxconn) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
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
