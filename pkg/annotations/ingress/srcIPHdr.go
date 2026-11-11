package ingress

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type SrcIPHdr struct {
	rules *rules.List
	name  string
}

func NewSrcIPHdr(n string, r *rules.List) *SrcIPHdr {
	return &SrcIPHdr{name: n, rules: r}
}

func (a *SrcIPHdr) GetName() string {
	return a.name
}

func (a *SrcIPHdr) Process(k store.K8s, annotations ...map[string]string) (err error) {
	if common.IsSet(a.name, annotations...) {
		utils.Once("deprecation-"+a.name, func() {
			logger.Warningf("DEPRECATED: annotation %q will be removed in a future version", a.name)
		})
	}
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		return err
	}
	input = strings.TrimSpace(input)
	if !common.ValidHeaderName(input) || strings.ContainsAny(input, "'$#") {
		return fmt.Errorf("incorrect header name in %s annotation", a.name)
	}
	a.rules.Add(&rules.ReqSetSrc{
		HeaderName: input,
	})
	return err
}
