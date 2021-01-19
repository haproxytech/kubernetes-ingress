package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqRateLimit struct {
	TableName      string
	ReqsLimit      int64
	DenyStatusCode int64
}

func (r ReqRateLimit) GetType() haproxy.RuleType {
	return haproxy.REQ_RATELIMIT
}

func (r ReqRateLimit) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
		//TODO: tcp request tracking
		return fmt.Errorf("request Track cannot be configured in TCP mode")
	}
	httpRule := models.HTTPRequestRule{
		Index:      utils.PtrInt64(0),
		Type:       "deny",
		DenyStatus: utils.PtrInt64(r.DenyStatusCode),
		Cond:       "if",
		CondTest:   fmt.Sprintf("{ sc0_http_req_rate(%s) gt %d }", r.TableName, r.ReqsLimit),
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL)
}
