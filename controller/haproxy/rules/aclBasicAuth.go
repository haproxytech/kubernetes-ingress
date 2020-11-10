package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqBasicAuth struct {
	Name    string
	Ingress haproxy.MapID
}

func (r ReqBasicAuth) GetType() haproxy.RuleType {
	return haproxy.REQ_AUTH
}

func (r ReqBasicAuth) Create(client api.HAProxyClient, frontend *models.Frontend) (err error) {
	httpReq := models.HTTPRequestRule{
		Type:      "auth",
		Index:     utils.PtrInt64(0),
		AuthRealm: r.Name,
		Cond:      models.HTTPRequestRuleCondIf,
		CondTest:  makeACL(fmt.Sprintf(" !{ http_auth_group(%s) authenticated-users } ", r.Name), r.Ingress.Path()),
	}
	if err = client.FrontendHTTPRequestRuleCreate(frontend.Name, httpReq); err != nil {
		return
	}

	return
}
