package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqPathRewrite struct {
	id        uint32
	PathMatch string
	PathFmt   string
}

func (r ReqPathRewrite) GetID() uint32 {
	if r.id == 0 {
		r.id = hashRule(r)
	}
	return r.id
}

func (r ReqPathRewrite) GetType() haproxy.RuleType {
	return haproxy.REQ_PATH_REWRITE
}

func (r ReqPathRewrite) Create(client api.HAProxyClient, frontend *models.Frontend) error {
	if frontend.Mode == "tcp" {
		return fmt.Errorf("SSL redirect cannot be configured in TCP mode")
	}
	httpRule := models.HTTPRequestRule{
		Index:     utils.PtrInt64(0),
		Type:      "replace-path",
		PathMatch: r.PathMatch,
		PathFmt:   r.PathFmt,
	}
	matchRuleID(&httpRule, r.GetID())
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule)
}
