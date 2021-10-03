package service

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type SSL struct {
	name    string
	backend *models.Backend
}

func NewSSL(n string, b *models.Backend) *SSL {
	return &SSL{name: n, backend: b}
}

func (a *SSL) GetName() string {
	return a.name
}

func (a *SSL) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	var enabled bool
	var err error
	if input != "" {
		enabled, err = utils.GetBoolValue(input, "server-ssl")
		if err != nil {
			return err
		}
	}
	if enabled {
		a.backend.DefaultServer.Ssl = "enabled"
		a.backend.DefaultServer.Alpn = "h2,http/1.1"
		a.backend.DefaultServer.Verify = "none"
	} else {
		a.backend.DefaultServer.Ssl = ""
		a.backend.DefaultServer.Alpn = ""
		a.backend.DefaultServer.Verify = ""
	}
	return nil
}
