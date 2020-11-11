package rules

import (
	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqCapture struct {
	id         uint32
	Expression string
	CaptureLen int64
}

func (r ReqCapture) GetID() uint32 {
	if r.id == 0 {
		r.id = hashRule(r)
	}
	return r.id
}

func (r ReqCapture) GetType() haproxy.RuleType {
	return haproxy.REQ_CAPTURE
}

func (r ReqCapture) Create(client api.HAProxyClient, frontend *models.Frontend) error {
	if frontend.Mode == "tcp" {
		tcpRule := models.TCPRequestRule{
			Index:      utils.PtrInt64(0),
			Type:       "content",
			Action:     "capture",
			CaptureLen: r.CaptureLen,
			Expr:       r.Expression,
		}
		matchRuleID(&tcpRule, r.GetID())
		return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule)
	}
	httpRule := models.HTTPRequestRule{
		Index:         utils.PtrInt64(0),
		Type:          "capture",
		CaptureSample: r.Expression,
		CaptureLen:    r.CaptureLen,
	}
	matchRuleID(&httpRule, r.GetID())
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule)
}
