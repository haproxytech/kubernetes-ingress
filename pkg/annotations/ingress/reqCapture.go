package ingress

import (
	"fmt"
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

func (a ReqCaptureAnn) Process(k store.K8s, annotations ...map[string]string) (err error) {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		return
	}

	switch a.name {
	case "request-capture":
		var reqCapture *rules.ReqCapture
		for _, sample := range strings.Split(input, "\n") {
			if sample == "" {
				continue
			}
			reqCapture = &rules.ReqCapture{
				Expression: sample,
			}
			a.parent.capture = append(a.parent.capture, reqCapture)
			a.parent.rules.Add(reqCapture)
		}
	case "request-capture-len":
		if len(a.parent.capture) == 0 {
			return
		}
		var captureLen int64
		captureLen, err = strconv.ParseInt(input, 10, 64)
		if err != nil {
			return
		}
		for _, rule := range a.parent.capture {
			rule.CaptureLen = captureLen
		}
	default:
		err = fmt.Errorf("unknown request-capture annotation '%s'", a.name)
	}
	return
}
