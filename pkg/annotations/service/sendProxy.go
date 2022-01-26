package service

import (
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type SendProxy struct {
	name    string
	backend *models.Backend
}

func NewSendProxy(n string, b *models.Backend) *SendProxy {
	return &SendProxy{name: n, backend: b}
}

func (a *SendProxy) GetName() string {
	return a.name
}

func (a *SendProxy) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	var proxyPorto string
	v := strings.ToLower(input)
	if v == "" {
		if a.backend.DefaultServer != nil {
			a.backend.DefaultServer.SendProxy = ""
			a.backend.DefaultServer.SendProxyV2 = ""
			a.backend.DefaultServer.SendProxyV2Ssl = ""
			a.backend.DefaultServer.SendProxyV2SslCn = ""
		}
		return nil
	} else if a.backend.DefaultServer == nil {
		a.backend.DefaultServer = &models.DefaultServer{}
	}
	switch v {
	case "proxy":
		a.backend.DefaultServer.SendProxy = "enabled"
	case "proxy-v1":
		a.backend.DefaultServer.SendProxy = "enabled"
	case "proxy-v2":
		a.backend.DefaultServer.SendProxyV2 = "enabled"
	case "proxy-v2-ssl":
		a.backend.DefaultServer.SendProxyV2Ssl = "enabled"
	case "proxy-v2-ssl-cn":
		a.backend.DefaultServer.SendProxyV2SslCn = "enabled"
	default:
		return fmt.Errorf("%s is an unknown enum", proxyPorto)
	}
	return nil
}
