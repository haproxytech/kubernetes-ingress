package ingress

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type ReqPathRewrite struct {
	rules *rules.List
	name  string
}

func NewReqPathRewrite(n string, r *rules.List) *ReqPathRewrite {
	return &ReqPathRewrite{name: n, rules: r}
}

func (a *ReqPathRewrite) GetName() string {
	return a.name
}

func (a *ReqPathRewrite) Process(k store.K8s, annotations ...map[string]string) (err error) {
	input := strings.TrimSpace(common.GetValue(a.GetName(), annotations...))
	if input == "" {
		return err
	}
	for _, rule := range strings.Split(input, "\n") {
		parts := strings.Fields(strings.TrimSpace(rule))
		for _, part := range parts {
			if _, ok := common.UnquotedWord(part); !ok {
				return fmt.Errorf("incorrect value '%s' in %s annotation", part, a.GetName())
			}
		}

		var rewrite *rules.ReqPathRewrite
		switch len(parts) {
		case 1:
			rewrite = &rules.ReqPathRewrite{
				PathMatch: "(.*)",
				PathFmt:   parts[0],
			}
		case 2:
			if !common.ValidRegex(common.RegexReplacePath, parts[0]) {
				return fmt.Errorf("incorrect value '%s' in %s annotation: HAProxy rejects the regex", parts[0], a.GetName())
			}
			rewrite = &rules.ReqPathRewrite{
				PathMatch: parts[0],
				PathFmt:   parts[1],
			}
		default:
			return fmt.Errorf("incorrect value '%s', path-rewrite takes 1 or 2 params ", input)
		}
		a.rules.Add(rewrite)
	}
	return err
}
