package rules

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ReqSetSrc struct {
	HeaderName string
}

func (r ReqSetSrc) GetType() Type {
	return REQ_SET_SRC
}

func (r ReqSetSrc) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if len(r.HeaderName) == 0 {
		return nil
	}
	if frontend.Mode == "tcp" {
		tcpRule := models.TCPRequestRule{
			Index: utils.PtrInt64(0),
			Type:  "set-src",
			Expr:  fmt.Sprintf("hdr(%s)", r.HeaderName),
		}
		return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule, ingressACL)
	}
	httpRule := models.HTTPRequestRule{
		Index: utils.PtrInt64(0),
		Type:  "set-src",
		Expr:  fmt.Sprintf("hdr(%s)", r.HeaderName),
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL)
}
