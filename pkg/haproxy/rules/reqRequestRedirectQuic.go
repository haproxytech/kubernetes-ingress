package rules

import (
	"errors"

	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type RequestRedirectQuic struct{}

func (r RequestRedirectQuic) GetType() Type {
	return REQ_REDIRECT
}

func (r RequestRedirectQuic) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
		return errors.New("request redirection cannot be configured in TCP mode")
	}

	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "redirect",
		Cond:       "unless",
		CondTest:   "{ ssl_fc }",
		RedirType:  "scheme",
		RedirValue: "https",
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL)
}
