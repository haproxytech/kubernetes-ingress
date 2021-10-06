package service

import (
	"strconv"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Maxconn struct {
	name   string
	server *models.Server
}

func NewMaxconn(n string, s *models.Server) *Maxconn {
	return &Maxconn{name: n, server: s}
}

func (a *Maxconn) GetName() string {
	return a.name
}

func (a *Maxconn) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		a.server.Maxconn = nil
		return nil
	}
	v, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return err
	}
	// adjust backend maxconn when using multiple HAProxy Instances
	if k.NbrHAProxyInst != 0 {
		v /= k.NbrHAProxyInst
	}
	a.server.Maxconn = &v
	return nil
}
