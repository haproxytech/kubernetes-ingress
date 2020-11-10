package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqProxyProtocol struct {
	SrcIPs haproxy.MapID
}

func (r ReqProxyProtocol) GetType() haproxy.RuleType {
	return haproxy.REQ_PROXY_PROTOCOL
}

func (r ReqProxyProtocol) Create(client api.HAProxyClient, frontend *models.Frontend) error {
	tcpRule := models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Type:     "connection",
		Action:   "expect-proxy layer4",
		Cond:     "if",
		CondTest: fmt.Sprintf("{ src %s }", r.SrcIPs.Path()),
	}
	return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule)
}
