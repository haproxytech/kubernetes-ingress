package rules

import (
	"fmt"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

type ReqSSLRedirect struct {
	Ingress      haproxy.MapID
	RedirectCode int64
}

func (r ReqSSLRedirect) GetType() haproxy.RuleType {
	return haproxy.REQ_SSL_REDIRECT
}

func (r ReqSSLRedirect) Create(client api.HAProxyClient, frontend *models.Frontend) error {
	if frontend.Mode == "tcp" {
		return fmt.Errorf("SSL redirect cannot be configured in TCP mode")
	}
	ingressMapFile := r.Ingress.Path()
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "redirect",
		RedirCode:  utils.PtrInt64(r.RedirectCode),
		RedirValue: "https",
		RedirType:  "scheme",
		Cond:       "if",
		CondTest:   makeACL(" !{ ssl_fc }", ingressMapFile),
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule)
}
