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
		return err
	}

	switch a.name {
	case "request-redirect":
		input = strings.TrimSpace(input)
		if input == "" || strings.ContainsAny(input, " {}\"'\\") {
			return fmt.Errorf("incorrect value in %s annotation: expected one unquoted token", a.name)
		}
		if keyword := common.TenantRestrictedKeyword(a.name, input, annotations...); keyword != "" {
			return fmt.Errorf("incorrect value in %s annotation: '%s' is not allowed", a.name, keyword)
		}
		host := strings.TrimPrefix(strings.TrimPrefix(input, HTTPS_PREFIX), HTTP_PREFIX)
		// Only a format expression can make HAProxy refuse the value.
		if strings.ContainsRune(host, '%') &&
			!common.ValidRule(common.FrontendRule, "http-request redirect location http://"+host+"%[capture.req.uri] code 302") {
			return fmt.Errorf("incorrect value in %s annotation: HAProxy rejects the expression", a.name)
		}
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
		return err
	case "request-redirect-code":
		if a.parent.redirect == nil {
			return err
		}
		var code int64
		code, err = strconv.ParseInt(input, 10, 64)
		if err != nil {
			return err
		}
		if !validRedirectCode(code) {
			return fmt.Errorf("incorrect value '%s' in %s annotation: HAProxy rejects the redirect code", input, a.name)
		}
		a.parent.redirect.RedirectCode = code
	default:
		err = fmt.Errorf("unknown redirect-redirect annotation '%s'", a.name)
	}
	return err
}

// validRedirectCode asks HAProxy, since the accepted codes depend on its version.
func validRedirectCode(code int64) bool {
	return common.ValidRule(common.FrontendRule, fmt.Sprintf("http-request redirect location / code %d", code))
}
