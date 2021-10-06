package ingress

import (
	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
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

func (a *SrcIPHdr) Process(k store.K8s, annotations ...map[string]string) (err error) {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		return
	}
	a.rules.Add(&rules.ReqSetSrc{
		HeaderName: input,
	})
	return
}
