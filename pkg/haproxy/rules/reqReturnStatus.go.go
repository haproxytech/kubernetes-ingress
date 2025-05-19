package rules

import (
	"errors"

	"github.com/haproxytech/client-native/v5/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
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
		Index:            utils.PtrInt64(0),
		ReturnStatusCode: &r.StatusCode,
		Type:             "return",
	}
	ingressACL += " METH_OPTIONS"
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL)
}
