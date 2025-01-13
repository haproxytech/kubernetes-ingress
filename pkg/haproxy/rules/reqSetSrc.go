package rules

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
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
			Type: "set-src",
			Expr: fmt.Sprintf("hdr(%s)", r.HeaderName),
		}
		return client.FrontendTCPRequestRuleCreate(0, frontend.Name, tcpRule, ingressACL)
	}
	httpRule := models.HTTPRequestRule{
		Type: "set-src",
		Expr: fmt.Sprintf("hdr(%s)", r.HeaderName),
	}
	ingressACL += " || !{ var(txn.path_match) -m found }"
	return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
}
