package annotations

import (
	"strings"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type ServerCookie struct {
	name    string
	enabled bool
	server  *models.Server
}

func NewServerCookie(n string, s *models.Server) *ServerCookie {
	return &ServerCookie{name: n, server: s}
}

func (a *ServerCookie) GetName() string {
	return a.name
}

func (a *ServerCookie) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.DELETED {
		return nil
	}
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	if len(strings.Fields(input.Value)) != 1 {
		// Error should already be reported in BackendCookie
		return nil
	}
	a.enabled = true
	return nil
}

func (a *ServerCookie) Update() error {
	if a.enabled {
		a.server.Cookie = a.server.Name
	} else {
		a.server.Cookie = ""
	}
	return nil
}
