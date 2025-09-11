package rules

import (
	"fmt"

	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
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
		return err
	}
	if !userList {
		err = client.UserListCreateByGroup(r.AuthGroup, r.Credentials)
		if err != nil {
			return err
		}
	}
	httpRule := models.HTTPRequestRule{
		Type:      "auth",
		AuthRealm: r.AuthRealm,
		Index:     utils.PtrInt64(0),
		Cond:      "if",
		CondTest:  fmt.Sprintf("!{ http_auth_group(%s) authenticated-users }", r.AuthGroup),
	}
	if err = client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL); err != nil {
		return err
	}

	return err
}
