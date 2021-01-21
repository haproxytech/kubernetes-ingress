package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type SetHdr struct {
	Response       bool
	ForwardedProto bool
	HdrName        string
	HdrFormat      string
	Type           haproxy.RuleType
	CondTest       string
}

func (r SetHdr) GetType() haproxy.RuleType {
	if r.ForwardedProto {
		return haproxy.REQ_FORWARDED_PROTO
	}
	if r.Response {
		return haproxy.RES_SET_HEADER
	}
	return haproxy.REQ_SET_HEADER
}

func (r SetHdr) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
		return fmt.Errorf("HTTP headers cannot be set in TCP mode")
	}
	// REQ_FORWARDED_PROTO
	if r.ForwardedProto {
		httpRule := models.HTTPRequestRule{
			Index:     utils.PtrInt64(0),
			Type:      "set-header",
			HdrName:   "X-Forwarded-Proto",
			HdrFormat: "https",
		}
		return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL)
	}
	// RES_SET_HEADER
	if r.Response {
		httpRule := models.HTTPResponseRule{
			Index:     utils.PtrInt64(0),
			Type:      "set-header",
			HdrName:   r.HdrName,
			HdrFormat: r.HdrFormat,
			CondTest:  r.CondTest,
		}
		return client.FrontendHTTPResponseRuleCreate(frontend.Name, httpRule, ingressACL)
	}
	// REQ_SET_HEADER
	httpRule := models.HTTPRequestRule{
		Index:     utils.PtrInt64(0),
		Type:      "set-header",
		HdrName:   r.HdrName,
		HdrFormat: r.HdrFormat,
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL)
}
