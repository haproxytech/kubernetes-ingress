package rules

import (
	"errors"

	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ReqAcceptContent struct{}

func (r ReqAcceptContent) GetType() Type {
	return REQ_ACCEPT_CONTENT
}

func (r ReqAcceptContent) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "http" {
		return errors.New("tcp accept-content rule is only available in TCP frontends")
	}
	tcpRule := models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Action:   "reject",
		Type:     "content",
		Cond:     "if",
		CondTest: "!{ req_ssl_hello_type 1 }",
	}
	return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule, ingressACL)
}
