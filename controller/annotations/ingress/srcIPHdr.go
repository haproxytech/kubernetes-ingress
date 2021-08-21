package ingress

import (
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
)

type SrcIPHdr struct {
	name  string
	rules *haproxy.Rules
}

func NewSrcIPHdr(n string, rules *haproxy.Rules) *SrcIPHdr {
	return &SrcIPHdr{name: n, rules: rules}
}

func (a *SrcIPHdr) GetName() string {
	return a.name
}

func (a *SrcIPHdr) Process(input string) (err error) {
	if input == "" {
		return
	}
	a.rules.Add(&rules.ReqSetSrc{
		HeaderName: input,
	})
	return
}
