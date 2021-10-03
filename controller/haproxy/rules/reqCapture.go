package rules

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
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
			Index:      utils.PtrInt64(0),
			Type:       "content",
			Action:     "capture",
			CaptureLen: r.CaptureLen,
			Expr:       r.Expression,
		}
		return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule, ingressACL)
	}
	httpRule := models.HTTPRequestRule{
		Index:         utils.PtrInt64(0),
		Type:          "capture",
		CaptureSample: r.Expression,
		CaptureLen:    r.CaptureLen,
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL)
}
