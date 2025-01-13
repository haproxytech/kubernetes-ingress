package rules

import (
	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
)

type ReqCapture struct {
	Expression string
	CaptureLen int64
}

func (r ReqCapture) GetType() Type {
	return REQ_CAPTURE
}

func (r ReqCapture) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
		tcpRule := models.TCPRequestRule{
			Type:       "content",
			Action:     "capture",
			CaptureLen: r.CaptureLen,
			Expr:       r.Expression,
		}
		return client.FrontendTCPRequestRuleCreate(0, frontend.Name, tcpRule, ingressACL)
	}
	httpRule := models.HTTPRequestRule{
		Type:          "capture",
		CaptureSample: r.Expression,
		CaptureLen:    r.CaptureLen,
	}
	return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
}
