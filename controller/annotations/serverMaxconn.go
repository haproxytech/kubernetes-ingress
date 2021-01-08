package annotations

import (
	"strconv"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type ServerMaxconn struct {
	name    string
	maxconn *int64
	server  *models.Server
}

func NewServerMaxconn(n string, s *models.Server) *ServerMaxconn {
	return &ServerMaxconn{name: n, server: s}
}

func (a *ServerMaxconn) GetName() string {
	return a.name
}

func (a *ServerMaxconn) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.DELETED {
		return nil
	}
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	v, err := strconv.ParseInt(input.Value, 10, 64)
	if err != nil {
		return err
	}
	a.maxconn = &v
	return nil
}

func (a *ServerMaxconn) Update() error {
	a.server.Maxconn = a.maxconn
	return nil
}
