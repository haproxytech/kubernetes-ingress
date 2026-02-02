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
	condTest := fmt.Sprintf("{ sc0_http_req_rate(%s) gt %d }", r.TableName, r.ReqsLimit)

	err := r.applyDefaults()
	if err != nil {
		return err
	}

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

	httpRule := models.HTTPRequestRule{
		Type:       "deny",
		DenyStatus: utils.PtrInt64(r.DenyStatusCode),
		Cond:       "if",
		CondTest:   condTest,
	}
	return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
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
