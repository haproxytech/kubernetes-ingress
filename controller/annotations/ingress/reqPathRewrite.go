package ingress

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
)

type ReqPathRewrite struct {
	name  string
	rules *haproxy.Rules
}

func NewReqPathRewrite(n string, rules *haproxy.Rules) *ReqPathRewrite {
	return &ReqPathRewrite{name: n, rules: rules}
}

func (a *ReqPathRewrite) GetName() string {
	return a.name
}

func (a *ReqPathRewrite) Process(input string) (err error) {
	if input == "" {
		return
	}
	parts := strings.Fields(strings.TrimSpace(input))

	var rewrite *rules.ReqPathRewrite
	switch len(parts) {
	case 1:
		rewrite = &rules.ReqPathRewrite{
			PathMatch: "(.*)",
			PathFmt:   parts[0],
		}
	case 2:
		rewrite = &rules.ReqPathRewrite{
			PathMatch: parts[0],
			PathFmt:   parts[1],
		}
	default:
		return fmt.Errorf("incorrect value '%s', path-rewrite takes 1 or 2 params ", input)
	}
	a.rules.Add(rewrite)
	return
}
