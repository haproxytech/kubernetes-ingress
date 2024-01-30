package rules

import (
	"fmt"

	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ReqRateLimit struct {
	TableName      string
	ReqsLimit      int64
	DenyStatusCode int64
}

func (r ReqRateLimit) GetType() Type {
	return REQ_RATELIMIT
}

func (r ReqRateLimit) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
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
