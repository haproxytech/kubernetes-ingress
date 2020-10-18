package rules

import (
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

type ReqSetVar struct {
	Name       string
	Scope      string
	Expression string
}

func (r ReqSetVar) GetType() haproxy.RuleType {
	return haproxy.REQ_SET_VAR
}

func (r ReqSetVar) Create(client api.HAProxyClient, frontend *models.Frontend) error {
	if frontend.Mode == "tcp" {
		tcpRule := models.TCPRequestRule{
			Index:    utils.PtrInt64(0),
			Type:     "content",
			Action:   "set-var",
			VarName:  r.Name,
			VarScope: r.Scope,
			Expr:     r.Expression,
		}
		return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule)
	}
	httpRule := models.HTTPRequestRule{
		Index:    utils.PtrInt64(0),
		Type:     "set-var",
		VarName:  r.Name,
		VarScope: r.Scope,
		VarExpr:  r.Expression,
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule)
}
