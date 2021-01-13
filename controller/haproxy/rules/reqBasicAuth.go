package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqBasicAuth struct {
	AuthGroup string
	Data      map[string][]byte
	id        uint32
}

func (r ReqBasicAuth) GetID() uint32 {
	if r.id == 0 {
		r.id = hashRule(r)
	}
	return r.id
}

func (r ReqBasicAuth) GetType() haproxy.RuleType {
	return haproxy.REQ_AUTH
}

func (r ReqBasicAuth) Create(client api.HAProxyClient, frontend *models.Frontend) (err error) {
	httpRule := models.HTTPRequestRule{
		Type:     "auth",
		Index:    utils.PtrInt64(0),
		Cond:     "if",
		CondTest: fmt.Sprintf("!{ http_auth_group(%s) authenticated-users }", r.AuthGroup),
	}
	matchRuleID(&httpRule, r.GetID())
	if err = client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule); err != nil {
		return
	}

	return
}
