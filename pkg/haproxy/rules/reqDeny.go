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
	cond, condTest := "", ""
	// An empty map denies access without depending on a pattern file.
	if r.SrcIPsMap != "" {
		not := ""
		if r.AllowList {
			not = "!"
		}
		cond = "if"
		condTest = fmt.Sprintf("%s{ src -f %s }", not, r.SrcIPsMap)
	}
	if frontend.Mode == "tcp" {
		tcpRule := models.TCPRequestRule{
			Type:     "content",
			Action:   "reject",
			Cond:     cond,
			CondTest: condTest,
		}
		return client.FrontendTCPRequestRuleCreate(0, frontend.Name, tcpRule, ingressACL)
	}
	httpRule := models.HTTPRequestRule{
		Type:       "deny",
		DenyStatus: utils.PtrInt64(403),
		Cond:       cond,
		CondTest:   condTest,
	}
	return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
}
