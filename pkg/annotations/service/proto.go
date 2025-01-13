package service

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type Proto struct {
	backend *models.Backend
	name    string
}

func NewProto(n string, b *models.Backend) *Proto {
	return &Proto{name: n, backend: b}
}

func (a *Proto) GetName() string {
	return a.name
}

func (a *Proto) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "h2" {
		if a.backend.DefaultServer == nil {
			a.backend.DefaultServer = &models.DefaultServer{}
		}
		a.backend.DefaultServer.Proto = "h2"
		return nil
	} else if a.backend.DefaultServer == nil {
		return nil
	}
	switch input {
	case "":
		a.backend.DefaultServer.Proto = ""
	case "h1":
		// Forces H1 even when SSL is enabled
		a.backend.DefaultServer.Alpn = ""
		a.backend.DefaultServer.Proto = ""
	default:
		return fmt.Errorf("unknown proto %s", input)
	}
	return nil
}
