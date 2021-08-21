package ingress

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type ReqAuth struct {
	authRule *rules.ReqBasicAuth
	rules    *haproxy.Rules
	k8s      store.K8s
	ingress  store.Ingress
}

type ReqAuthAnn struct {
	name   string
	parent *ReqAuth
}

func NewReqAuth(rules *haproxy.Rules, i store.Ingress, k store.K8s) *ReqAuth {
	return &ReqAuth{rules: rules, ingress: i, k8s: k}
}

func (p *ReqAuth) NewAnnotation(n string) ReqAuthAnn {
	return ReqAuthAnn{name: n, parent: p}
}

func (a ReqAuthAnn) GetName() string {
	return a.name
}

func (a ReqAuthAnn) Process(input string) (err error) {
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
		secret, err = a.parent.k8s.FetchSecret(input, a.parent.ingress.Namespace)
		if err != nil {
			return err
		}
		if secret.Status == store.DELETED {
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
