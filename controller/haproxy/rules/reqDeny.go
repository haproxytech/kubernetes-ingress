package rules

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqDeny struct {
	SrcIPsMap string
	Whitelist bool
}

func (r ReqDeny) GetType() haproxy.RuleType {
	return haproxy.REQ_DENY
}

func (r ReqDeny) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	srcIpsMap := haproxy.GetMapPath(r.SrcIPsMap)
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
			CondTest: fmt.Sprintf("%s{ src -f %s }", not, srcIpsMap),
		}
		return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule, ingressACL)
	}
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "deny",
		DenyStatus: utils.PtrInt64(403),
		Cond:       "if",
		CondTest:   fmt.Sprintf("%s{ src -f %s }", not, srcIpsMap),
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL)
}
