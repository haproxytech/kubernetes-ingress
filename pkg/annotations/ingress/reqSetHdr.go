package ingress

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type SetHdr struct {
	rules    *rules.List
	name     string
	response bool
}

func NewReqSetHdr(n string, r *rules.List) *SetHdr {
	return &SetHdr{name: n, rules: r}
}

func NewResSetHdr(n string, r *rules.List) *SetHdr {
	return &SetHdr{name: n, rules: r, response: true}
}

func (a *SetHdr) GetName() string {
	return a.name
}

func (a *SetHdr) Process(k store.K8s, annotations ...map[string]string) (err error) {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		return err
	}
	for _, param := range strings.Split(input, "\n") {
		if param == "" {
			continue
		}
		indexSpace := strings.IndexByte(param, ' ')
		if indexSpace == -1 {
			return fmt.Errorf("incorrect value in %s annotation", a.name)
		}
		hdrName, hdrValue := param[:indexSpace], param[indexSpace+1:]
		if !common.ValidHeaderName(hdrName) || strings.ContainsAny(hdrName, "'$#") {
			return fmt.Errorf("incorrect header name '%s' in %s annotation", hdrName, a.name)
		}
		if !safeHeaderValue(hdrValue) {
			return fmt.Errorf("incorrect value for header '%s' in %s annotation", hdrName, a.name)
		}
		hdrFormat := common.EnsureQuoted(hdrValue)
		direction := "http-request"
		if a.response {
			direction = "http-response"
		}
		// Only a format expression can make HAProxy refuse the value.
		if strings.ContainsRune(hdrFormat, '%') && !common.ValidRule(common.FrontendRule, direction+" set-header "+hdrName+" "+hdrFormat) {
			return fmt.Errorf("incorrect value for header '%s' in %s annotation: HAProxy rejects the expression", hdrName, a.name)
		}
		a.rules.Add(&rules.SetHdr{
			HdrName:   hdrName,
			HdrFormat: hdrFormat,
			Response:  a.response,
		})
	}
	return err
}

// safeHeaderValue reports whether EnsureQuoted keeps the value in one
// non-empty quoted token. Inside quotes, '\' escapes, '$' expands the
// environment, and an embedded quote ends the token early.
func safeHeaderValue(value string) bool {
	if common.HasControlChar(value) || strings.ContainsAny(value, `\$`) {
		return false
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	// HAProxy reads an empty quoted argument as the end of the line.
	return value != "" && !strings.ContainsRune(value, '"')
}
