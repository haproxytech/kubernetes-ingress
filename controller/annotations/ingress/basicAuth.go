package ingress

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type ReqAuth struct {
	authRule *rules.ReqBasicAuth
	rules    *haproxy.Rules
	ingress  store.Ingress
}

type ReqAuthAnn struct {
	name   string
	parent *ReqAuth
}

func NewReqAuth(rules *haproxy.Rules, i store.Ingress) *ReqAuth {
	return &ReqAuth{rules: rules, ingress: i}
}

func (p *ReqAuth) NewAnnotation(n string) ReqAuthAnn {
	return ReqAuthAnn{name: n, parent: p}
}

func (a ReqAuthAnn) GetName() string {
	return a.name
}

func (a ReqAuthAnn) Process(k store.K8s, annotations ...map[string]string) (err error) {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		return
	}

	switch a.name {
	case "auth-type":
		if input != "basic-auth" {
			return fmt.Errorf("incorrect auth-type value '%s'. Only 'basic-auth' value is currently supported", input)
		}
		authGroup := "Global"
		if a.parent.ingress.Namespace != "" {
			authGroup = fmt.Sprintf("%s-%s", a.parent.ingress.Namespace, a.parent.ingress.Name)
		}
		a.parent.authRule = &rules.ReqBasicAuth{
			AuthGroup: authGroup,
			AuthRealm: "Protected-Content",
		}
		a.parent.rules.Add(a.parent.authRule)
	case "auth-realm":
		if a.parent.authRule == nil {
			return
		}
		a.parent.authRule.AuthRealm = strings.ReplaceAll(input, " ", "-")
	case "auth-secret":
		if a.parent.authRule == nil {
			return
		}
		var secret *store.Secret
		ns, name, errAnn := common.GetK8sPath(a.name, annotations...)
		if errAnn != nil {
			err = errAnn
			return
		}
		if ns == "" {
			ns = a.parent.ingress.Namespace
		}
		secret, _ = k.GetSecret(ns, name)
		if secret == nil {
			return
		}
		a.parent.authRule.Credentials = make(map[string][]byte)
		for u, pwd := range secret.Data {
			if pwd[len(pwd)-1] == '\n' {
				// logger.Warningf("Ingress %s/%s: basic-auth: password for user %s ends with '\\n'. Ignoring last character.", ingress.Namespace, ingress.Name, u)
				pwd = pwd[:len(pwd)-1]
			}
			a.parent.authRule.Credentials[u] = pwd
		}
	default:
		err = fmt.Errorf("unknown auth-type annotation '%s'", a.name)
	}
	return
}
