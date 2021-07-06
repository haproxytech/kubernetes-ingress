package annotations

import (
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v2/models"
)

type ServerSendProxy struct {
	name       string
	proxyPorto string
	server     *models.Server
}

func NewServerSendProxy(n string, s *models.Server) *ServerSendProxy {
	return &ServerSendProxy{name: n, server: s}
}

func (a *ServerSendProxy) GetName() string {
	return a.name
}

func (a *ServerSendProxy) Parse(input string) error {
	v := strings.ToLower(input)
	switch v {
	case "proxy", "proxy-v1", "proxy-v2":
		a.proxyPorto = v
	default:
		return fmt.Errorf("%s is an unknown enum", input)
	}
	return nil
}

func (a *ServerSendProxy) Update() error {
	switch a.proxyPorto {
	case "proxy":
		a.server.SendProxy = "enabled"
	case "proxy-v1":
		a.server.SendProxy = "enabled"
	case "proxy-v2":
		a.server.SendProxyV2 = "enabled"
	case "":
		a.server.SendProxy = ""
		a.server.SendProxyV2 = ""
	default:
		return fmt.Errorf("%s is an unknown enum", a.proxyPorto)
	}
	return nil
}
