package annotations

import (
	"strconv"

	"github.com/haproxytech/client-native/v2/models"
)

type ServerMaxconn struct {
	name   string
	server *models.Server
}

func NewServerMaxconn(n string, s *models.Server) *ServerMaxconn {
	return &ServerMaxconn{name: n, server: s}
}

func (a *ServerMaxconn) GetName() string {
	return a.name
}

func (a *ServerMaxconn) Process(input string) error {
	if input == "" {
		a.server.Maxconn = nil
		return nil
	}
	v, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return err
	}
	a.server.Maxconn = &v
	return nil
}
