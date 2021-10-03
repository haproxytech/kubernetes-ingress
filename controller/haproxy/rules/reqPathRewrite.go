package rules

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqPathRewrite struct {
	PathMatch string
	PathFmt   string
}

func (r ReqPathRewrite) GetType() Type {
	return REQ_PATH_REWRITE
}

func (r ReqPathRewrite) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
		return fmt.Errorf("SSL redirect cannot be configured in TCP mode")
	}
	httpRule := models.HTTPRequestRule{
		Index:     utils.PtrInt64(0),
		Type:      "replace-path",
		PathMatch: r.PathMatch,
		PathFmt:   r.PathFmt,
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL)
}
