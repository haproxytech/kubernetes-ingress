package annotations

import (
	"strconv"

	"github.com/haproxytech/client-native/v2/models"
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

func (a *ServerMaxconn) Parse(input string) error {
	v, err := strconv.ParseInt(input, 10, 64)
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
