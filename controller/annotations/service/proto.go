package service

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Proto struct {
	name   string
	server *models.Server
}

func NewProto(n string, s *models.Server) *Proto {
	return &Proto{name: n, server: s}
}

func (a *Proto) GetName() string {
	return a.name
}

func (a *Proto) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	switch input {
	case "":
		a.server.Proto = ""
	case "h1":
		// Forces H1 even when SSL is enabled
		a.server.Alpn = ""
		a.server.Proto = ""
	case "h2":
		a.server.Proto = "h2"
	default:
		return fmt.Errorf("unknown proto %s", input)
	}
	return nil
}
