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
	AuthRealm string
	Data      map[string][]byte
}

func (r ReqBasicAuth) GetType() haproxy.RuleType {
	return haproxy.REQ_AUTH
}

func (r ReqBasicAuth) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) (err error) {
	httpRule := models.HTTPRequestRule{
		Type:      "auth",
		AuthRealm: r.AuthRealm,
		Index:     utils.PtrInt64(0),
		Cond:      "if",
		CondTest:  fmt.Sprintf("!{ http_auth_group(%s) authenticated-users }", r.AuthGroup),
	}
	if err = client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL); err != nil {
		return
	}

	return
}
