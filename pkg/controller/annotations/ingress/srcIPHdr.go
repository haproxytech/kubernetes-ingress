package ingress

import (
	"github.com/haproxytech/kubernetes-ingress/pkg/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type SrcIPHdr struct {
	name  string
	rules *rules.Rules
}

func NewSrcIPHdr(n string, r *rules.Rules) *SrcIPHdr {
	return &SrcIPHdr{name: n, rules: r}
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
