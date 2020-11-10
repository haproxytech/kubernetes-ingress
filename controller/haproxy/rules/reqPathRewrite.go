package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqPathRewrite struct {
	Ingress   haproxy.MapID
	PathMatch string
	PathFmt   string
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
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule)
}
