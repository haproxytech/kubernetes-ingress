package annotations

import (
	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ServerCheck struct {
	name    string
	enabled bool
	server  *models.Server
}

func NewServerCheck(n string, s *models.Server) *ServerCheck {
	return &ServerCheck{name: n, server: s}
}

func (a *ServerCheck) GetName() string {
	return a.name
}

func (a *ServerCheck) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.DELETED {
		return nil
	}
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	var err error
	a.enabled, err = utils.GetBoolValue(input.Value, "check")
	return err
}

func (a *ServerCheck) Update() error {
	if a.enabled {
		a.server.Check = "enabled"
	} else {
		a.server.Check = "disabled"
	}
	return nil
}
