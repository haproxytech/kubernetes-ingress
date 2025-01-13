package rules

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ReqDeny struct {
	SrcIPsMap maps.Path
	AllowList bool
}

func (r ReqDeny) GetType() Type {
	return REQ_DENY
}

func (r ReqDeny) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	not := ""
	if r.AllowList {
		not = "!"
	}
	if frontend.Mode == "tcp" {
		tcpRule := models.TCPRequestRule{
			Type:     "content",
			Action:   "reject",
			Cond:     "if",
			CondTest: fmt.Sprintf("%s{ src -f %s }", not, r.SrcIPsMap),
		}
		return client.FrontendTCPRequestRuleCreate(0, frontend.Name, tcpRule, ingressACL)
	}
	httpRule := models.HTTPRequestRule{
		Type:       "deny",
		DenyStatus: utils.PtrInt64(403),
		Cond:       "if",
		CondTest:   fmt.Sprintf("%s{ src -f %s }", not, r.SrcIPsMap),
	}
	return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
}
