package rules

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
)

type ReqProxyProtocol struct {
	SrcIPsMap maps.Path
}

func (r ReqProxyProtocol) GetType() Type {
	return REQ_PROXY_PROTOCOL
}

func (r ReqProxyProtocol) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	tcpRule := models.TCPRequestRule{
		Type:     "connection",
		Action:   models.TCPRequestRuleActionExpectDashProxy,
		Cond:     "if",
		CondTest: fmt.Sprintf("{ src -f %s }", r.SrcIPsMap),
	}
	return client.FrontendTCPRequestRuleCreate(0, frontend.Name, tcpRule, ingressACL)
}
