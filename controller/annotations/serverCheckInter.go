package annotations

import (
	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ServerCheckInter struct {
	name   string
	inter  int64
	server *models.Server
}

func NewServerCheckInter(n string, s *models.Server) *ServerCheckInter {
	return &ServerCheckInter{name: n, server: s}
}

func (a *ServerCheckInter) GetName() string {
	return a.name
}

func (a *ServerCheckInter) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.DELETED {
		return nil
	}
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	value, err := utils.ParseTime(input.Value)
	if err != nil {
		return err
	}
	a.inter = *value
	return nil
}

func (a *ServerCheckInter) Update() error {
	a.server.Inter = &a.inter
	return nil
}
