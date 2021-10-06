package service

import (
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type SendProxy struct {
	name   string
	server *models.Server
}

func NewSendProxy(n string, s *models.Server) *SendProxy {
	return &SendProxy{name: n, server: s}
}

func (a *SendProxy) GetName() string {
	return a.name
}

func (a *SendProxy) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
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
