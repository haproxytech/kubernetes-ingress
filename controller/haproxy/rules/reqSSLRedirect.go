package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqSSLRedirect struct {
	id           uint32
	RedirectCode int64
	RedirectPort int
}

func (r ReqSSLRedirect) GetID() uint32 {
	if r.id == 0 {
		r.id = hashRule(r)
	}
	return r.id
}

func (r ReqSSLRedirect) GetType() haproxy.RuleType {
	return haproxy.REQ_SSL_REDIRECT
}

func (r ReqSSLRedirect) Create(client api.HAProxyClient, frontend *models.Frontend) error {
	if frontend.Mode == "tcp" {
		return fmt.Errorf("SSL redirect cannot be configured in TCP mode")
	}
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "redirect",
		RedirCode:  utils.PtrInt64(r.RedirectCode),
		RedirValue: fmt.Sprintf("https://%%[hdr(host),field(1,:)]:%d%%[capture.req.uri]", r.RedirectPort),
		RedirType:  "location",
	}
	matchRuleID(&httpRule, r.GetID())
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule)
}
