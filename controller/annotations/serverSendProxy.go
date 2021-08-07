package annotations

import (
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v2/models"
)

type ServerSendProxy struct {
	name   string
	server *models.Server
}

func NewServerSendProxy(n string, s *models.Server) *ServerSendProxy {
	return &ServerSendProxy{name: n, server: s}
}

func (a *ServerSendProxy) GetName() string {
	return a.name
}

func (a *ServerSendProxy) Process(input string) error {
	var proxyPorto string
	v := strings.ToLower(input)
	switch v {
	case "proxy":
		a.server.SendProxy = "enabled"
	case "proxy-v1":
		a.server.SendProxy = "enabled"
	case "proxy-v2":
		a.server.SendProxyV2 = "enabled"
	case "proxy-v2-ssl":
		a.server.SendProxyV2Ssl = "enabled"
	case "proxy-v2-ssl-cn":
		a.server.SendProxyV2SslCn = "enabled"
	case "":
		a.server.SendProxy = ""
		a.server.SendProxyV2 = ""
		a.server.SendProxyV2Ssl = ""
		a.server.SendProxyV2SslCn = ""
	default:
		return fmt.Errorf("%s is an unknown enum", proxyPorto)
	}
	return nil
}
