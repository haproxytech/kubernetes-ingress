package service

import (
	"strconv"

	"github.com/haproxytech/client-native/v2/models"
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

func (a *Maxconn) Process(input string) error {
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
