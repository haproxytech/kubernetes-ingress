package ingress

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ReqSetHost struct {
	rules *rules.List
	name  string
}

func NewReqSetHost(n string, r *rules.List) *ReqSetHost {
	return &ReqSetHost{name: n, rules: r}
}

func (a *ReqSetHost) GetName() string {
	return a.name
}

func (a *ReqSetHost) Process(k store.K8s, annotations ...map[string]string) (err error) {
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
	if input == "" || strings.ContainsAny(input, " {}\"'\\") {
		return fmt.Errorf("incorrect value in %s annotation: expected one unquoted token", a.name)
	}
	if keyword := common.TenantRestrictedKeyword(a.name, input, annotations...); keyword != "" {
		return fmt.Errorf("incorrect value in %s annotation: '%s' is not allowed", a.name, keyword)
	}
	// Only a format expression can make HAProxy refuse the value.
	if strings.ContainsRune(input, '%') && !common.ValidRule(common.FrontendRule, "http-request set-header Host "+input) {
		return fmt.Errorf("incorrect value in %s annotation: HAProxy rejects the expression", a.name)
	}
	a.rules.Add(&rules.SetHdr{
		HdrName:   "Host",
		HdrFormat: input,
	})
	return err
}
