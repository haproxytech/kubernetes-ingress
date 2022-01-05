package rules

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ReqProxyProtocol struct {
	SrcIPsMap maps.Path
}

func (r ReqProxyProtocol) GetType() Type {
	return REQ_PROXY_PROTOCOL
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
