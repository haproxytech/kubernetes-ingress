package rules

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqProxyProtocol struct {
	SrcIPsMap maps.Path
}

func (r ReqProxyProtocol) GetType() haproxy.RuleType {
	return haproxy.REQ_PROXY_PROTOCOL
}

func (r ReqProxyProtocol) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	tcpRule := models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Type:     "connection",
		Action:   models.TCPRequestRuleActionExpectProxy,
		Cond:     "if",
		CondTest: fmt.Sprintf("{ src -f %s }", r.SrcIPsMap),
	}
	return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule, ingressACL)
}
