package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqProxyProtocol struct {
	SrcIPsMap string
}

func (r ReqProxyProtocol) GetType() haproxy.RuleType {
	return haproxy.REQ_PROXY_PROTOCOL
}

func (r ReqProxyProtocol) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	tcpRule := models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Type:     "connection",
		Action:   "expect-proxy layer4",
		Cond:     "if",
		CondTest: fmt.Sprintf("{ src -f %s }", haproxy.GetMapPath(r.SrcIPsMap)),
	}
	return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule, ingressACL)
}
