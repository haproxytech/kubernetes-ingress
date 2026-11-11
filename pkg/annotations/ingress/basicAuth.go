package ingress

import (
	"errors"
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ReqAuth struct {
	authRule *rules.ReqBasicAuth
	rules    *rules.List
	ingress  *store.Ingress
}

type ReqAuthAnn struct {
	parent *ReqAuth
	name   string
}

func NewReqAuth(rules *rules.List, i *store.Ingress) *ReqAuth {
	return &ReqAuth{rules: rules, ingress: i}
}

func (p *ReqAuth) NewAnnotation(n string) ReqAuthAnn {
	return ReqAuthAnn{name: n, parent: p}
}

func (a ReqAuthAnn) GetName() string {
	return a.name
}

func (a ReqAuthAnn) Process(k store.K8s, annotations ...map[string]string) (err error) {
	if common.IsSet(a.name, annotations...) {
		utils.Once("deprecation-"+a.name, func() {
			logger.Warningf("DEPRECATED: annotation %q will be removed in a future version", a.name)
		})
	}
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		return err
	}

	switch a.name {
	case "auth-type":
		if input != "basic-auth" {
			return fmt.Errorf("incorrect auth-type value '%s'. Only 'basic-auth' value is currently supported", input)
		}
		authGroup := "Global"
		if a.parent.ingress != nil {
			authGroup = fmt.Sprintf("%s-%s", a.parent.ingress.Namespace, a.parent.ingress.Name)
		}
		a.parent.authRule = &rules.ReqBasicAuth{
			AuthGroup: authGroup,
			AuthRealm: "Protected-Content",
		}
		a.parent.rules.Add(a.parent.authRule)
	case "auth-realm":
		if a.parent.authRule == nil {
			return err
		}
		if common.HasControlChar(input) || strings.ContainsAny(input, `"'\{}$`) {
			return errors.New("incorrect auth-realm value")
		}
		a.parent.authRule.AuthRealm = strings.ReplaceAll(input, " ", "-")
	case "auth-secret":
		if a.parent.authRule == nil {
			return err
		}
		var secret *store.Secret
		ns, name, errAnn := common.GetK8sPath(a.name, annotations...)
		if errAnn != nil {
			err = errAnn
			return err
		}
		if ns == "" {
			if a.parent.ingress == nil {
				err = fmt.Errorf("auth-secret '%s' has no namespace outside an ingress", name)
				return err
			}
			ns = a.parent.ingress.Namespace
		}
		secret, _ = k.GetSecret(ns, name)
		if secret == nil {
			return err
		}
		a.parent.authRule.Credentials = make(map[string][]byte)
		for u, pwd := range secret.Data {
			// If the pwd length is 0, we do not create the user in the userlist
			// This is not a valid setting
			if len(pwd) > 0 {
				if pwd[len(pwd)-1] == '\n' {
					pwd = pwd[:len(pwd)-1]
				}
				if errCred := credentialError(u, pwd); errCred != nil {
					logger.Warningf("Secret '%s/%s': auth-secret: ignoring user '%s': %s", ns, name, u, errCred)
					continue
				}
				a.parent.authRule.Credentials[u] = pwd
			}
		}
	default:
		err = fmt.Errorf("unknown auth-type annotation '%s'", a.name)
	}
	return err
}

// badCredentialChar rejects separators the unescaped userlist line would interpret.
func badCredentialChar(s string) (rune, bool) {
	for _, r := range s {
		if r <= ' ' || r == 0x7f || r == '#' || r == '"' || r == '\'' || r == '\\' {
			return r, true
		}
	}
	return 0, false
}

// credentialError names the field whose value would break the userlist line.
func credentialError(name string, pwd []byte) error {
	if len(pwd) == 0 {
		return errors.New("empty password")
	}
	if r, bad := badCredentialChar(name); bad {
		return fmt.Errorf("invalid character %q in user name", r)
	}
	if r, bad := badCredentialChar(string(pwd)); bad {
		return fmt.Errorf("invalid character %q in password", r)
	}
	return nil
}
