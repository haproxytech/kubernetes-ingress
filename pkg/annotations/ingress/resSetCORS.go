package ingress

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

const corsVarName = "cors_origin"

type ResSetCORS struct {
	rules   *rules.List
	methods map[string]struct{}
	acl     string
}

type ResSetCORSAnn struct {
	parent *ResSetCORS
	name   string
}

func NewResSetCORS(r *rules.List) *ResSetCORS {
	return &ResSetCORS{
		rules:   r,
		methods: map[string]struct{}{"GET": {}, "POST": {}, "PUT": {}, "DELETE": {}, "HEAD": {}, "CONNECT": {}, "OPTIONS": {}, "TRACE": {}, "PATCH": {}},
	}
}

func (p *ResSetCORS) NewAnnotation(n string) ResSetCORSAnn {
	return ResSetCORSAnn{
		name:   n,
		parent: p,
	}
}

func (a ResSetCORSAnn) GetName() string {
	return a.name
}

func (a ResSetCORSAnn) Process(k store.K8s, annotations ...map[string]string) (err error) {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		return nil
	}

	switch a.name {
	case "cors-enable":
		var enabled bool
		enabled, err = utils.GetBoolValue(input, "cors-enable")
		if !enabled {
			return err
		}
		// SetVar rule to capture Origin header
		a.parent.rules.Add(&rules.ReqSetVar{
			Name:       corsVarName,
			Scope:      "txn",
			Expression: "req.hdr(origin)",
		})
		a.parent.acl = fmt.Sprintf("{ var(txn.%s) -m found }", corsVarName)
	case "cors-allow-origin":
		if a.parent.acl == "" {
			return err
		}
		// Access-Control-Allow-Origin = *
		origin := "*"
		if input != "*" {
			// Access-Control-Allow-Origin = <origin>
			// only origins in the annotation
			a.parent.acl = fmt.Sprintf("{ var(txn.%s) -m reg %s }", corsVarName, input)
			origin = "%[var(txn." + corsVarName + ")]"
		}
		a.parent.rules.Add(&rules.SetHdr{
			HdrName:       "Access-Control-Allow-Origin",
			HdrFormat:     origin,
			AfterResponse: true,
			CondTest:      a.parent.acl,
			Cond:          "if",
		})
	case "cors-allow-methods":
		if a.parent.acl == "" {
			return err
		}
		if input != "*" {
			input = strings.Join(strings.Fields(input), "") // strip spaces
			methods := strings.Split(input, ",")
			for i, method := range methods {
				methods[i] = strings.ToUpper(method)
				if _, ok := a.parent.methods[methods[i]]; !ok {
					return fmt.Errorf("unsupported HTTP method '%s' in cors-allow-methods configuration", methods[i])
				}
			}
			input = common.EnsureQuoted(strings.Join(methods, ", "))
		}
		a.parent.rules.Add(&rules.SetHdr{
			HdrName:       "Access-Control-Allow-Methods",
			HdrFormat:     input,
			AfterResponse: true,
			CondTest:      a.parent.acl,
			Cond:          "if",
		})
	case "cors-allow-headers":
		if a.parent.acl == "" {
			return err
		}
		input = strings.Join(strings.Fields(input), "") // strip spaces
		a.parent.rules.Add(rules.SetHdr{
			HdrName:       "Access-Control-Allow-Headers",
			HdrFormat:     common.EnsureQuoted(input),
			AfterResponse: true,
			CondTest:      a.parent.acl,
			Cond:          "if",
		})
	case "cors-max-age":
		if a.parent.acl == "" {
			return err
		}
		var duration *int64
		duration, err = utils.ParseTime(input)
		if err != nil {
			return err
		}
		maxage := *duration / 1000
		if maxage < -1 {
			return fmt.Errorf("invalid cors-max-age value %d", maxage)
		}
		a.parent.rules.Add(&rules.SetHdr{
			HdrName:       "Access-Control-Max-Age",
			HdrFormat:     fmt.Sprintf("\"%d\"", maxage),
			AfterResponse: true,
			CondTest:      a.parent.acl,
			Cond:          "if",
		})
	case "cors-allow-credentials":
		if a.parent.acl == "" || input != "true" {
			return err
		}
		a.parent.rules.Add(&rules.SetHdr{
			HdrName:       "Access-Control-Allow-Credentials",
			HdrFormat:     "\"true\"",
			AfterResponse: true,
			CondTest:      a.parent.acl,
			Cond:          "if",
		})
	case "cors-respond-to-options":
		if a.parent.acl == "" || input != "true" {
			return err
		}
		a.parent.rules.Add(&rules.ReqReturnStatus{
			StatusCode: 204,
		})
	default:
		err = fmt.Errorf("unknown cors annotation '%s'", a.name)
	}
	return err
}
