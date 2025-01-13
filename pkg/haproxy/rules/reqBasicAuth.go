package rules

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
)

type ReqBasicAuth struct {
	Credentials map[string][]byte
	AuthGroup   string
	AuthRealm   string
}

func (r ReqBasicAuth) GetType() Type {
	return REQ_AUTH
}

func (r ReqBasicAuth) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) (err error) {
	var userList bool
	userList, err = client.UserListExistsByGroup(r.AuthGroup)
	if err != nil {
		return
	}
	if !userList {
		err = client.UserListCreateByGroup(r.AuthGroup, r.Credentials)
		if err != nil {
			return
		}
	}
	httpRule := models.HTTPRequestRule{
		Type:      "auth",
		AuthRealm: r.AuthRealm,
		Cond:      "if",
		CondTest:  fmt.Sprintf("!{ http_auth_group(%s) authenticated-users }", r.AuthGroup),
	}
	if err = client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL); err != nil {
		return
	}

	return
}
