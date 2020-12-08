package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqDeny struct {
	id        uint32
	SrcIPsMap string
	Whitelist bool
}

func (r ReqDeny) GetID() uint32 {
	if r.id == 0 {
		r.id = hashRule(r)
	}
	return r.id
}

func (r ReqDeny) GetType() haproxy.RuleType {
	return haproxy.REQ_DENY
}

func (r ReqDeny) Create(client api.HAProxyClient, frontend *models.Frontend) error {
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
		matchRuleID(&tcpRule, r.GetID())
		return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule)
	}
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "deny",
		DenyStatus: utils.PtrInt64(403),
		Cond:       "if",
		CondTest:   fmt.Sprintf("%s{ src -f %s }", not, srcIpsMap),
	}
	matchRuleID(&httpRule, r.GetID())
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule)
}
