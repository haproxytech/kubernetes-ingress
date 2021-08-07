package annotations

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ServerCheckInter struct {
	name   string
	server *models.Server
}

func NewServerCheckInter(n string, s *models.Server) *ServerCheckInter {
	return &ServerCheckInter{name: n, server: s}
}

func (a *ServerCheckInter) GetName() string {
	return a.name
}

func (a *ServerCheckInter) Process(input string) error {
	if input == "" {
		a.server.Inter = nil
		return nil
	}
	value, err := utils.ParseTime(input)
	if err != nil {
		return err
	}
	a.server.Inter = value
	return nil
}
