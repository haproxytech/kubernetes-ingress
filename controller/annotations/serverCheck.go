package annotations

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ServerCheck struct {
	name   string
	server *models.Server
}

func NewServerCheck(n string, s *models.Server) *ServerCheck {
	return &ServerCheck{name: n, server: s}
}

func (a *ServerCheck) GetName() string {
	return a.name
}

func (a *ServerCheck) Process(input string) error {
	if input == "" {
		a.server.Check = ""
		return nil
	}
	enabled, err := utils.GetBoolValue(input, "check")
	if err != nil {
		return err
	}
	if enabled {
		a.server.Check = "enabled"
	} else {
		a.server.Check = "disabled"
	}
	return nil
}
