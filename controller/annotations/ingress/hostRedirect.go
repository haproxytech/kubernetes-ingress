package ingress

import (
	"fmt"
	"strconv"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type HostRedirect struct {
	redirect *rules.RequestRedirect
	rules    *haproxy.Rules
}

type HostRedirectAnn struct {
	name   string
	parent *HostRedirect
}

func NewHostRedirect(rules *haproxy.Rules) *HostRedirect {
	return &HostRedirect{rules: rules}
}

func (p *HostRedirect) NewAnnotation(n string) HostRedirectAnn {
	return HostRedirectAnn{
		name:   n,
		parent: p,
	}
}

func (a HostRedirectAnn) GetName() string {
	return a.name
}

func (a HostRedirectAnn) Process(k store.K8s, annotations ...map[string]string) (err error) {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		return
	}

	switch a.name {
	case "request-redirect":
		a.parent.redirect = &rules.RequestRedirect{Host: input}
		a.parent.rules.Add(a.parent.redirect)
		return
	case "request-redirect-code":
		if a.parent.redirect == nil {
			return
		}
		var code int64
		code, err = strconv.ParseInt(input, 10, 64)
		if err != nil {
			return
		}
		a.parent.redirect.RedirectCode = code
	default:
		err = fmt.Errorf("unknown redirect-redirect annotation '%s'", a.name)
	}
	return
}
