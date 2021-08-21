package ingress

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
)

type SetHdr struct {
	name     string
	rules    *haproxy.Rules
	response bool
}

func NewReqSetHdr(n string, rules *haproxy.Rules) *SetHdr {
	return &SetHdr{name: n, rules: rules}
}

func NewResSetHdr(n string, rules *haproxy.Rules) *SetHdr {
	return &SetHdr{name: n, rules: rules, response: true}
}

func (a *SetHdr) GetName() string {
	return a.name
}

func (a *SetHdr) Process(input string) (err error) {
	if input == "" {
		return
	}
	for _, param := range strings.Split(input, "\n") {
		if param == "" {
			continue
		}
		indexSpace := strings.IndexByte(param, ' ')
		if indexSpace == -1 {
			return fmt.Errorf("incorrect value '%s' in request-set-header annotation", param)
		}
		a.rules.Add(&rules.SetHdr{
			HdrName:   param[:indexSpace],
			HdrFormat: "\"" + param[indexSpace+1:] + "\"",
			Response:  a.response,
		})
	}
	return
}
