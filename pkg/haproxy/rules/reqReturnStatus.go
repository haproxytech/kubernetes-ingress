package rules

import (
	"errors"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
)

//nolint:golint,stylecheck
var MIME_TYPE_TEXT_PLAIN string = "text/plain"

type ReqReturnStatus struct {
	StatusCode int64
}

func (r ReqReturnStatus) GetType() Type {
	return REQ_RETURN_STATUS
}

func (r ReqReturnStatus) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
		return errors.New("HTTP status cannot be set in TCP mode")
	}
	httpRule := models.HTTPRequestRule{
		ReturnStatusCode: &r.StatusCode,
		Type:             "return",
	}
	ingressACL += " METH_OPTIONS"
	return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
}
