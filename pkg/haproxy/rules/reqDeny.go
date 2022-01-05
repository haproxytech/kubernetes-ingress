package rules

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ReqDeny struct {
	SrcIPsMap maps.Path
	Whitelist bool
}

func (r ReqDeny) GetType() Type {
	return REQ_DENY
}

func (r ReqDeny) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	not := ""
	if r.Whitelist {
		not = "!"
	}
	if frontend.Mode == "tcp" {
		tcpRule := models.TCPRequestRule{
			Index:    utils.PtrInt64(0),
			Type:     "content",
			Action:   "reject",
			Cond:     "if",
			CondTest: fmt.Sprintf("%s{ src -f %s }", not, r.SrcIPsMap),
		}
		return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule, ingressACL)
	}
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "deny",
		DenyStatus: utils.PtrInt64(403),
		Cond:       "if",
		CondTest:   fmt.Sprintf("%s{ src -f %s }", not, r.SrcIPsMap),
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL)
}
