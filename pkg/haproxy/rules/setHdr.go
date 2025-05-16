package rules

import (
	"errors"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
)

type SetHdr struct {
	HdrName        string
	HdrFormat      string
	CondTest       string
	Cond           string
	Type           Type
	Response       bool
	AfterResponse  bool
	ForwardedProto bool
}

func (r SetHdr) GetType() Type {
	if r.ForwardedProto {
		return REQ_FORWARDED_PROTO
	}
	if r.Response {
		return RES_SET_HEADER
	}
	return REQ_SET_HEADER
}

func (r SetHdr) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
		return errors.New("HTTP headers cannot be set in TCP mode")
	}
	// REQ_FORWARDED_PROTO
	if r.ForwardedProto {
		httpRule := models.HTTPRequestRule{
			Type:      "set-header",
			HdrName:   "X-Forwarded-Proto",
			HdrFormat: "https",
		}
		return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
	}
	// RES_SET_HEADER
	if r.Response {
		httpRule := models.HTTPResponseRule{
			Type:      "set-header",
			HdrName:   r.HdrName,
			HdrFormat: r.HdrFormat,
			CondTest:  r.CondTest,
			Cond:      r.Cond,
		}
		return client.FrontendHTTPResponseRuleCreate(0, frontend.Name, httpRule, ingressACL)
	}

	if r.AfterResponse {
		httpRule := models.HTTPAfterResponseRule{
			Type:      "set-header",
			HdrName:   r.HdrName,
			HdrFormat: r.HdrFormat,
			CondTest:  r.CondTest,
			Cond:      r.Cond,
		}
		return client.FrontendHTTPAfterResponseRuleCreate(0, frontend.Name, httpRule, ingressACL)
	}
	// REQ_SET_HEADER
	httpRule := models.HTTPRequestRule{
		Type:      "set-header",
		HdrName:   r.HdrName,
		HdrFormat: r.HdrFormat,
	}
	return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
}
