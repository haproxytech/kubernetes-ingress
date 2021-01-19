package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqAcceptContent struct {
}

func (r ReqAcceptContent) GetType() haproxy.RuleType {
	return haproxy.REQ_ACCEPT_CONTENT
}

func (r ReqAcceptContent) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "http" {
		return fmt.Errorf("tcp accept-content rule is only available in TCP frontends")
	}
	tcpRule := models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Action:   "accept",
		Type:     "content",
		Cond:     "if",
		CondTest: "{ req_ssl_hello_type 1 }",
	}
	return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule, ingressACL)
}
