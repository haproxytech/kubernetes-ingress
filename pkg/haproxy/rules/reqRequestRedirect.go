package rules

import (
	"errors"
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type RequestRedirect struct {
	Host         string
	RedirectCode int64
	RedirectPort int
	SSLRequest   bool
	SSLRedirect  bool
}

func (r RequestRedirect) GetType() Type {
	return REQ_REDIRECT
}

func (r RequestRedirect) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
		return errors.New("request redirection cannot be configured in TCP mode")
	}
	var httpRule models.HTTPRequestRule
	if r.SSLRedirect {
		httpRule = r.sslRedirect()
	} else {
		httpRule = r.hostRedirect()
	}

	return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
}

func (r RequestRedirect) sslRedirect() models.HTTPRequestRule {
	rule := fmt.Sprintf("https://%%[hdr(host),field(1,:)]:%d%%[capture.req.uri]", r.RedirectPort)
	httpRule := models.HTTPRequestRule{
		Type:       "redirect",
		RedirCode:  utils.PtrInt64(r.RedirectCode),
		RedirValue: rule,
		RedirType:  "location",
	}

	return httpRule
}

func (r RequestRedirect) hostRedirect() models.HTTPRequestRule {
	scheme := "http"
	if r.SSLRequest {
		scheme = "https"
	}
	rule := fmt.Sprintf(scheme+"://%s%%[capture.req.uri]", r.Host)
	httpRule := models.HTTPRequestRule{
		Type:       "redirect",
		RedirCode:  utils.PtrInt64(r.RedirectCode),
		RedirValue: rule,
		RedirType:  "location",
	}
	return httpRule
}
