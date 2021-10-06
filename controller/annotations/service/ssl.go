package service

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type SSL struct {
	name   string
	server *models.Server
}

func NewSSL(n string, s *models.Server) *SSL {
	return &SSL{name: n, server: s}
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
		a.server.Ssl = "enabled"
		a.server.Alpn = "h2,http/1.1"
		a.server.Verify = "none"
	} else {
		a.server.Ssl = ""
		a.server.Alpn = ""
		a.server.Verify = ""
	}
	return nil
}
