package rules

import (
	"errors"
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ReqRateLimit struct {
	TableName      string
	ReqsLimit      int64
	DenyStatusCode int64
	WhitelistIPs   []string    // Direct IPs and CIDRs
	WhitelistMaps  []maps.Path // Pattern file references
	ExcludePathEnd []string    // Path suffixes excluded from rate limiting
}

const (
	defaultRateLimitStatueCode = "403"
)

func (r ReqRateLimit) GetType() Type {
	return REQ_RATELIMIT
}

func (r ReqRateLimit) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
		return errors.New("request Track cannot be configured in TCP mode")
	}

	// ReqsLimit == 0 means rate-limit disabled
	if r.ReqsLimit == 0 {
		return nil
	}

	err := r.applyDefaults()
	if err != nil {
		return err
	}

	httpRule := models.HTTPRequestRule{
		Type:       "deny",
		DenyStatus: utils.PtrInt64(r.DenyStatusCode),
		Cond:       "if",
		CondTest:   r.condTest(),
	}
	return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
}

// condTest builds the HAProxy condition of the deny rule: the rate check,
// optionally narrowed by the source whitelist and the path-suffix exclusion.
func (r ReqRateLimit) condTest() string {
	condTest := fmt.Sprintf("{ sc0_http_req_rate(%s) gt %d }", r.TableName, r.ReqsLimit)

	// Build whitelist conditions if configured
	// If whitelist is set, only apply rate limiting if source IP is NOT in the whitelist
	if len(r.WhitelistIPs) > 0 || len(r.WhitelistMaps) > 0 {
		var whitelistConditions []string

		// Add direct IP/CIDR condition
		if len(r.WhitelistIPs) > 0 {
			whitelistConditions = append(whitelistConditions,
				fmt.Sprintf("!{ src %s }", strings.Join(r.WhitelistIPs, " ")))
		}

		// Add pattern file conditions
		for _, mapPath := range r.WhitelistMaps {
			whitelistConditions = append(whitelistConditions,
				fmt.Sprintf("!{ src -f %s }", mapPath))
		}

		condTest = fmt.Sprintf("%s %s", condTest, strings.Join(whitelistConditions, " "))
	}

	// Never deny requests whose path ends with an excluded suffix. The
	// matching ReqTrack rule skips them too, so they are neither counted
	// nor rate limited.
	if len(r.ExcludePathEnd) > 0 {
		condTest = fmt.Sprintf("%s !{ path_end %s }", condTest, strings.Join(r.ExcludePathEnd, " "))
	}

	return condTest
}

func (r *ReqRateLimit) applyDefaults() error {
	if r.DenyStatusCode == 0 {
		code, err := utils.ParseInt(defaultRateLimitStatueCode)
		if err != nil {
			return err
		}
		r.DenyStatusCode = code
	}
	return nil
}
