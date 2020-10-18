package rules

import (
	"fmt"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

type ReqDeny struct {
	Ingress   haproxy.MapID
	SrcIPs    haproxy.MapID
	Whitelist bool
}

func (r ReqDeny) GetType() haproxy.RuleType {
	return haproxy.REQ_DENY
}

func (r ReqDeny) Create(client api.HAProxyClient, frontend *models.Frontend) error {
	ingressMapFile := r.Ingress.Path()
	ipsMapFile := r.SrcIPs.Path()
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
			CondTest: fmt.Sprintf("{ req_ssl_sni -f %s } %s{ src -f %s }", ingressMapFile, not, ipsMapFile),
		}
		return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule)
	}
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "deny",
		DenyStatus: 403,
		Cond:       "if",
		CondTest:   makeACL(fmt.Sprintf(" %s{ src -f %s }", not, ipsMapFile), ingressMapFile),
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule)
}
