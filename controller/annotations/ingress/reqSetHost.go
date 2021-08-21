package ingress

import (
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
)

type ReqSetHost struct {
	name  string
	rules *haproxy.Rules
}

func NewReqSetHost(n string, rules *haproxy.Rules) *ReqSetHost {
	return &ReqSetHost{name: n, rules: rules}
}

func (a *ReqSetHost) GetName() string {
	return a.name
}

func (a *ReqSetHost) Process(input string) (err error) {
	if input == "" {
		return
	}
	a.rules.Add(&rules.SetHdr{
		HdrName:   "Host",
		HdrFormat: input,
	})
	return
}
