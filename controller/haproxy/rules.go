package haproxy

import (
	"fmt"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

type Rule interface {
	GetType() RuleType
	Create(client api.HAProxyClient, frontend *models.Frontend) error
}

// Order matters !
// Rules will be evaluated by HAProxy in the defined order.
type RuleType int

//nolint
const (
	REQ_ACCEPT_CONTENT RuleType = iota
	REQ_INSPECT_DELAY
	REQ_PROXY_PROTOCOL
	REQ_SET_SRC
	REQ_SET_VAR
	REQ_DENY
	REQ_TRACK
	REQ_AUTH
	REQ_RATELIMIT
	REQ_CAPTURE
	REQ_SSL_REDIRECT
	REQ_FORWARDED_PROTO
	REQ_SET_HEADER
	REQ_SET_HOST
	REQ_PATH_REWRITE
	RES_SET_HEADER
)

type Rules map[string]*frontendRules

// This structure may evolve with additional fields
type frontendRules struct {
	rules map[RuleType][]Rule
}

var logger = utils.GetLogger()

func NewRules() Rules {
	return make(map[string]*frontendRules)
}

func (r Rules) AddRule(rule Rule, frontends ...string) error {
	if rule == nil || len(frontends) == 0 {
		return fmt.Errorf("invalid params")
	}
	for _, frontend := range frontends {
		ftRules, ok := r[frontend]
		if !ok {
			ftRules = &frontendRules{
				rules: make(map[RuleType][]Rule),
			}
			r[frontend] = ftRules
		}
		ruleType := rule.GetType()
		ftRules.rules[ruleType] = append(ftRules.rules[ruleType], rule)
	}
	return nil
}

func (r Rules) EnableSSLPassThrough(passThroughFtd, offloadFtd string) {
	if _, ok := r[offloadFtd]; !ok {
		return
	}
	if _, ok := r[passThroughFtd]; !ok {
		r[passThroughFtd] = &frontendRules{
			rules: make(map[RuleType][]Rule),
		}
	}
	for _, ruleType := range []RuleType{REQ_PROXY_PROTOCOL, REQ_DENY} {
		r[passThroughFtd].rules[ruleType] = r[offloadFtd].rules[ruleType]
		delete(r[offloadFtd].rules, ruleType)
	}
}

func (r Rules) Refresh(client api.HAProxyClient) (reload bool) {
	for feName, feRules := range r {
		fe, err := client.FrontendGet(feName)
		if err != nil {
			logger.Error(err)
			continue
		}
		client.FrontendRuleDeleteAll(feName)
		// All rules are created with Index 0,
		// Which means first rule inserted will be last in the list of HAProxy rules after iteration
		// Thus iteration is done in reverse to preserve order between the defined rules in
		// controller and the resulting order in HAProxy configuration.
		for ruleType := RES_SET_HEADER; ruleType >= REQ_ACCEPT_CONTENT; ruleType-- {
			ruleSet := feRules.rules[ruleType]
			for i := len(ruleSet) - 1; i >= 0; i-- {
				if err := ruleSet[i].Create(client, &fe); err == nil {
					reload = true
				} else {
					logger.Error(err)
				}
			}
		}
	}
	return reload
}
