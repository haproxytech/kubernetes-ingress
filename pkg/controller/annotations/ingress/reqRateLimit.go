package ingress

import (
	"fmt"
	"strconv"

	"github.com/haproxytech/kubernetes-ingress/pkg/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ReqRateLimit struct {
	limit *rules.ReqRateLimit
	track *rules.ReqTrack
	rules *rules.Rules
}

type ReqRateLimitAnn struct {
	name   string
	parent *ReqRateLimit
}

func NewReqRateLimit(r *rules.Rules) *ReqRateLimit {
	return &ReqRateLimit{rules: r}
}

func (p *ReqRateLimit) NewAnnotation(n string) ReqRateLimitAnn {
	return ReqRateLimitAnn{
		name:   n,
		parent: p,
	}
}

func (a ReqRateLimitAnn) GetName() string {
	return a.name
}

func (a ReqRateLimitAnn) Process(k store.K8s, annotations ...map[string]string) (err error) {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		return nil
	}

	switch a.name {
	case "rate-limit-requests":
		// Enable Ratelimiting
		var value int64
		value, err = strconv.ParseInt(input, 10, 64)
		a.parent.limit = &rules.ReqRateLimit{ReqsLimit: value}
		a.parent.track = &rules.ReqTrack{TrackKey: "src"}
		a.parent.rules.Add(a.parent.limit)
		a.parent.rules.Add(a.parent.track)
	case "rate-limit-period":
		if a.parent.limit == nil || a.parent.track == nil {
			return
		}
		var value *int64
		value, err = utils.ParseTime(input)
		tableName := fmt.Sprintf("RateLimit-%d", *value)
		a.parent.track.TablePeriod = value
		a.parent.track.TableName = tableName
		a.parent.limit.TableName = tableName
	case "rate-limit-size":
		if a.parent.limit == nil || a.parent.track == nil {
			return
		}
		var value *int64
		value, err = utils.ParseSize(input)
		a.parent.track.TableSize = value
	case "rate-limit-status-code":
		if a.parent.limit == nil || a.parent.track == nil {
			return
		}
		var value int64
		value, err = utils.ParseInt(input)
		a.parent.limit.DenyStatusCode = value
	default:
		err = fmt.Errorf("unknown rate-limit annotation '%s'", a.name)
	}
	return
}
