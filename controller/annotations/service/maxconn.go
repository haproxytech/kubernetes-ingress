package service

import (
	"strconv"

	"github.com/haproxytech/client-native/v2/models"
)

type Maxconn struct {
	name   string
	pods   int64 // haproxy pods
	server *models.Server
}

func NewMaxconn(n string, s *models.Server, pods int64) *Maxconn {
	if pods == 0 {
		pods = 1
	}
	return &Maxconn{name: n, pods: pods, server: s}
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
	v /= a.pods
	a.server.Maxconn = &v
	return nil
}
