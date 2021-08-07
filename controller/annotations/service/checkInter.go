package service

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type CheckInter struct {
	name   string
	server *models.Server
}

func NewCheckInter(n string, s *models.Server) *CheckInter {
	return &CheckInter{name: n, server: s}
}

func (a *CheckInter) GetName() string {
	return a.name
}

func (a *CheckInter) Process(input string) error {
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
