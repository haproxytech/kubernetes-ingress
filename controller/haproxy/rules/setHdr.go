package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type SetHdr struct {
	id             uint32
	Response       bool
	ForwardedProto bool
	HdrName        string
	HdrFormat      string
	Type           haproxy.RuleType
}

func (r SetHdr) GetID() uint32 {
	if r.id == 0 {
		r.id = hashRule(r)
	}
	return r.id
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

func (r SetHdr) Create(client api.HAProxyClient, frontend *models.Frontend) error {
	if frontend.Mode == "tcp" {
		return fmt.Errorf("HTTP headers cannot be set in TCP mode")
	}
	//REQ_FORWARDED_PROTO
	if r.ForwardedProto {
		httpRule := models.HTTPRequestRule{
			Index:     utils.PtrInt64(0),
			Type:      "set-header",
			HdrName:   "X-Forwarded-Proto",
			HdrFormat: "https",
		}
		return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule)
	}
	//RES_SET_HEADER
	if r.Response {
		httpRule := models.HTTPResponseRule{
			Index:     utils.PtrInt64(0),
			Type:      "set-header",
			HdrName:   r.HdrName,
			HdrFormat: r.HdrFormat,
		}
		matchRuleID(&httpRule, r.GetID())
		return client.FrontendHTTPResponseRuleCreate(frontend.Name, httpRule)
	}
	//REQ_SET_HEADER
	httpRule := models.HTTPRequestRule{
		Index:     utils.PtrInt64(0),
		Type:      "set-header",
		HdrName:   r.HdrName,
		HdrFormat: r.HdrFormat,
	}
	matchRuleID(&httpRule, r.GetID())
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule)
}
