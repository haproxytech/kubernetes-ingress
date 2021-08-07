package annotations

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ServerSSL struct {
	name   string
	server *models.Server
}

func NewServerSSL(n string, s *models.Server) *ServerSSL {
	return &ServerSSL{name: n, server: s}
}

func (a *ServerSSL) GetName() string {
	return a.name
}

func (a *ServerSSL) Process(input string) error {
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
