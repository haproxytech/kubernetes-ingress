package ingress

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type ReqCapture struct {
	rules   *rules.List
	capture []*rules.ReqCapture
}

type ReqCaptureAnn struct {
	parent *ReqCapture
	name   string
}

func NewReqCapture(r *rules.List) *ReqCapture {
	return &ReqCapture{rules: r}
}

func (p *ReqCapture) NewAnnotation(n string) ReqCaptureAnn {
	return ReqCaptureAnn{
		name:   n,
		parent: p,
	}
}

func (a ReqCaptureAnn) GetName() string {
	return a.name
}

var captureExpression = regexp.MustCompile(`^[A-Za-z0-9_.+-]+(?:\(.*\))?(?:,[A-Za-z0-9_.+-]+(?:\(.*\))?)*$`)

func validCaptureExpression(sample string) bool {
	words := strings.FieldsFunc(sample, func(r rune) bool { return r == ' ' })
	for i, word := range words {
		if _, ok := common.UnquotedWord(word); !ok || !captureExpression.MatchString(word) {
			return false
		}
		// Later words must not become capture or condition directives.
		if i > 0 && (word == "len" || word == "id" || word == "if" || word == "unless") {
			return false
		}
	}
	return len(words) > 0
}

func (a ReqCaptureAnn) Process(k store.K8s, annotations ...map[string]string) (err error) {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		return err
	}

	switch a.name {
	case "request-capture":
		var captures []*rules.ReqCapture
		for _, sample := range strings.Split(input, "\n") {
			if sample == "" {
				continue
			}
			if !validCaptureExpression(sample) {
				return fmt.Errorf("incorrect value '%s' in %s annotation", sample, a.GetName())
			}
			if keyword := common.TenantRestrictedKeyword(a.name, sample, annotations...); keyword != "" {
				return fmt.Errorf("incorrect value '%s' in %s annotation: '%s' is not allowed", sample, a.GetName(), keyword)
			}
			if !common.ValidRule(common.FrontendRule, "http-request capture "+sample+" len 1") {
				return fmt.Errorf("incorrect value '%s' in %s annotation: HAProxy rejects the expression", sample, a.GetName())
			}
			captures = append(captures, &rules.ReqCapture{Expression: sample})
		}
		for _, capture := range captures {
			a.parent.capture = append(a.parent.capture, capture)
			a.parent.rules.Add(capture)
		}
	case "request-capture-len":
		if len(a.parent.capture) == 0 {
			return err
		}
		var captureLen int64
		captureLen, err = strconv.ParseInt(input, 10, 64)
		if err != nil {
			return err
		}
		if !common.ValidRule(common.FrontendRule, fmt.Sprintf("http-request capture src len %d", captureLen)) {
			return fmt.Errorf("incorrect value '%s' in %s annotation: HAProxy rejects the length", input, a.GetName())
		}
		for _, rule := range a.parent.capture {
			rule.CaptureLen = captureLen
		}
	default:
		err = fmt.Errorf("unknown request-capture annotation '%s'", a.name)
	}
	return err
}
