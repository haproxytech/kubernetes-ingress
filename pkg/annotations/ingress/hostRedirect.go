package ingress

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

//nolint:golint,stylecheck
const (
	HTTP_PREFIX  = "http://"
	HTTPS_PREFIX = "https://"
)

type HostRedirect struct {
	redirect *rules.RequestRedirect
	rules    *rules.List
}

type HostRedirectAnn struct {
	parent *HostRedirect
	name   string
}

func NewHostRedirect(r *rules.List) *HostRedirect {
	return &HostRedirect{rules: r}
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
		switch {
		case strings.HasPrefix(input, HTTPS_PREFIX):
			a.parent.redirect.SSLRequest = true
			a.parent.redirect.Host = a.parent.redirect.Host[len(HTTPS_PREFIX):]
		case strings.HasPrefix(input, HTTP_PREFIX):
			a.parent.redirect.SSLRequest = false
			a.parent.redirect.Host = a.parent.redirect.Host[len(HTTP_PREFIX):]
		}
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
