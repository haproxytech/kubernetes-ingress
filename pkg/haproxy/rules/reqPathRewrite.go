package rules

import (
	"errors"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
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
		return errors.New("SSL redirect cannot be configured in TCP mode")
	}
	httpRule := models.HTTPRequestRule{
		Type:      "replace-path",
		PathMatch: r.PathMatch,
		PathFmt:   r.PathFmt,
	}
	return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
}
