package ingress

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

const corsVarName = "cors_origin"

type ResSetCORS struct {
	rules   *rules.Rules
	acl     string
	methods map[string]struct{}
}

type ResSetCORSAnn struct {
	name   string
	parent *ResSetCORS
}

func NewResSetCORS(r *rules.Rules) *ResSetCORS {
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
			return
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
			return
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
			HdrName:   "Access-Control-Allow-Origin",
			HdrFormat: origin,
			Response:  true,
			CondTest:  a.parent.acl,
			Cond:      "if",
		})
	case "cors-allow-methods":
		if a.parent.acl == "" {
			return
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
			input = "\"" + strings.Join(methods, ", ") + "\""
		}
		a.parent.rules.Add(&rules.SetHdr{
			HdrName:   "Access-Control-Allow-Methods",
			HdrFormat: input,
			Response:  true,
			CondTest:  a.parent.acl,
			Cond:      "if",
		})
	case "cors-allow-headers":
		if a.parent.acl == "" {
			return
		}
		input = strings.Join(strings.Fields(input), "") // strip spaces
		a.parent.rules.Add(rules.SetHdr{
			HdrName:   "Access-Control-Allow-Headers",
			HdrFormat: "\"" + input + "\"",
			Response:  true,
			CondTest:  a.parent.acl,
			Cond:      "if",
		})
	case "cors-max-age":
		if a.parent.acl == "" {
			return
		}
		var duration *int64
		duration, err = utils.ParseTime(input)
		if err != nil {
			return
		}
		maxage := *duration / 1000
		if maxage < -1 {
			return fmt.Errorf("invalid cors-max-age value %d", maxage)
		}
		a.parent.rules.Add(&rules.SetHdr{
			HdrName:   "Access-Control-Max-Age",
			HdrFormat: fmt.Sprintf("\"%d\"", maxage),
			Response:  true,
			CondTest:  a.parent.acl,
			Cond:      "if",
		})
	case "cors-allow-credentials":
		if a.parent.acl == "" || input != "true" {
			return
		}
		a.parent.rules.Add(&rules.SetHdr{
			HdrName:   "Access-Control-Allow-Credentials",
			HdrFormat: "\"true\"",
			Response:  true,
			CondTest:  a.parent.acl,
			Cond:      "if",
		})
	default:
		err = fmt.Errorf("unknown cors annotation '%s'", a.name)
	}
	return
}
