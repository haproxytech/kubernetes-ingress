package service

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Proto struct {
	name    string
	backend *models.Backend
}

func NewProto(n string, b *models.Backend) *Proto {
	return &Proto{name: n, backend: b}
}

func (a *Proto) GetName() string {
	return a.name
}

func (a *Proto) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	switch input {
	case "":
		a.backend.DefaultServer.Proto = ""
	case "h1":
		// Forces H1 even when SSL is enabled
		a.backend.DefaultServer.Alpn = ""
		a.backend.DefaultServer.Proto = ""
	case "h2":
		a.backend.DefaultServer.Proto = "h2"
	default:
		return fmt.Errorf("unknown proto %s", input)
	}
	return nil
}
