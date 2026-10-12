package ingress

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// excludePathEndEntry validates path suffixes passed to
// rate-limit-exclude-path-end so annotation values cannot inject arbitrary
// HAProxy configuration into the generated ACL conditions.
var excludePathEndEntry = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

type ReqRateLimit struct {
	limit *rules.ReqRateLimit
	track *rules.ReqTrack
	rules *rules.List
	maps  maps.Maps
}

type ReqRateLimitAnn struct {
	parent *ReqRateLimit
	name   string
}

func NewReqRateLimit(r *rules.List, m maps.Maps) *ReqRateLimit {
	return &ReqRateLimit{rules: r, maps: m}
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
			return errors.New("rate-limit-period requires rate-limit-requests to be set")
		}
		var value *int64
		value, err = utils.ParseTime(input)
		tableName := fmt.Sprintf("RateLimit-%d", *value)
		a.parent.track.TablePeriod = value
		a.parent.track.TableName = tableName
		a.parent.limit.TableName = tableName
	case "rate-limit-size":
		if a.parent.limit == nil || a.parent.track == nil {
			return errors.New("rate-limit-size requires rate-limit-requests to be set")
		}
		var value *int64
		value, err = utils.ParseSize(input)
		a.parent.track.TableSize = value
	case "rate-limit-status-code":
		if a.parent.limit == nil || a.parent.track == nil {
			return errors.New("rate-limit-status-code requires rate-limit-requests to be set")
		}
		var value int64
		value, err = utils.ParseInt(input)
		a.parent.limit.DenyStatusCode = value
	case "rate-limit-whitelist":
		if a.parent.limit == nil || a.parent.track == nil {
			return errors.New("rate-limit-whitelist requires rate-limit-requests to be set")
		}

		// Parse the input - can be:
		// 1. Comma-separated IPs/CIDRs
		// 2. One or more pattern file references (patterns/file1, patterns/file2)
		// 3. Mix of both

		var ips []string
		var patterns []maps.Path

		for _, entry := range strings.Split(input, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}

			// Check if it's a pattern file reference
			if strings.HasPrefix(entry, "patterns/") {
				patterns = append(patterns, maps.Path(entry))
			} else {
				// Validate it's a valid IP or CIDR
				if ip := net.ParseIP(entry); ip == nil {
					if _, _, err := net.ParseCIDR(entry); err != nil {
						return fmt.Errorf("incorrect address '%s' in %s annotation", entry, a.name)
					}
				}
				ips = append(ips, entry)
			}
		}

		// Store IPs/CIDRs directly in the rule
		a.parent.limit.WhitelistIPs = ips

		// Store pattern file references
		a.parent.limit.WhitelistMaps = patterns
	case "rate-limit-exclude-path-end":
		if a.parent.limit == nil || a.parent.track == nil {
			return errors.New("rate-limit-exclude-path-end requires rate-limit-requests to be set")
		}

		var values []string
		for _, source := range annotations {
			value, found := source[a.name]
			if !found || value == "" {
				continue
			}
			if extended := strings.TrimPrefix(value, "+"); extended != value {
				values = append(values, extended)
				continue
			}
			values = append(values, value)
			break
		}
		input = strings.Join(values, " ")

		var suffixes []string
		for _, entry := range strings.FieldsFunc(input, func(r rune) bool { return r == ',' || r == ' ' }) {
			if !excludePathEndEntry.MatchString(entry) {
				return fmt.Errorf("incorrect path suffix '%s' in %s annotation", entry, a.name)
			}
			suffixes = append(suffixes, entry)
		}
		if len(suffixes) == 0 {
			return fmt.Errorf("no path suffixes in %s annotation", a.name)
		}

		// Excluded requests are neither tracked nor denied: they don't
		// increment the rate counter and are never rate limited.
		a.parent.track.ExcludePathEnd = suffixes
		a.parent.limit.ExcludePathEnd = suffixes
	default:
		err = fmt.Errorf("unknown rate-limit annotation '%s'", a.name)
	}
	return err
}
