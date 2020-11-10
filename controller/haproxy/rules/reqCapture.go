package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqCapture struct {
	Ingress    haproxy.MapID
	Expression string
	CaptureLen int64
}

func (r ReqCapture) GetType() haproxy.RuleType {
	return haproxy.REQ_CAPTURE
}

func (r ReqCapture) Create(client api.HAProxyClient, frontend *models.Frontend) error {
	ingressMapFile := r.Ingress.Path()
	if frontend.Mode == "http" {
		tcpRule := models.TCPRequestRule{
			Index:      utils.PtrInt64(0),
			Type:       "content",
			Action:     "capture",
			CaptureLen: r.CaptureLen,
			Expr:       r.Expression,
			Cond:       "if",
			CondTest:   fmt.Sprintf("{ req_ssl_sni -f %s }", ingressMapFile),
		}
		return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule)
	}
	httpRule := models.HTTPRequestRule{
		Index:         utils.PtrInt64(0),
		Type:          "capture",
		CaptureSample: r.Expression,
		Cond:          "if",
		CaptureLen:    r.CaptureLen,
		CondTest:      makeACL("", ingressMapFile),
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule)
}
