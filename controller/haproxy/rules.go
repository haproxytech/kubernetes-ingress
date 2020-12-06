package haproxy

import (
	"fmt"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

type Rule interface {
	GetID() uint32
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

type RuleStatus int

//nolint
const (
	TO_DELETE RuleStatus = iota
	TO_CREATE
	CREATED
)

type Rules map[string]*frontendRules

type frontendRules struct {
	rules  map[RuleType][]Rule
	status map[uint32]RuleStatus
}

var logger = utils.GetLogger()

func NewRules() Rules {
	return make(map[string]*frontendRules)
}

func (r Rules) AddRule(rule Rule, frontends ...string) error {
	if rule == nil || len(frontends) == 0 {
		return fmt.Errorf("invalid params")
	}
	ruleID := rule.GetID()
	ruleType := rule.GetType()
	for _, frontend := range frontends {
		ftRules, ok := r[frontend]
		if !ok {
			ftRules = &frontendRules{
				rules:  make(map[RuleType][]Rule),
				status: make(map[uint32]RuleStatus),
			}
			r[frontend] = ftRules
		}
		if _, ok := ftRules.status[ruleID]; ok {
			ftRules.status[ruleID] = CREATED
		} else {
			ftRules.rules[ruleType] = append(ftRules.rules[ruleType], rule)
			ftRules.status[ruleID] = TO_CREATE
		}
	}
	return nil
}

func (r Rules) EnableSSLPassThrough(passThroughFtd, offloadFtd string) {
	if _, ok := r[offloadFtd]; !ok {
		return
	}
	if _, ok := r[passThroughFtd]; !ok {
		r[passThroughFtd] = &frontendRules{
			rules:  make(map[RuleType][]Rule),
			status: make(map[uint32]RuleStatus),
		}
	}
	// Move some layer 4 rules from sslOffloading Frontend to sslPassthrough Frontend
	for _, ruleType := range []RuleType{REQ_PROXY_PROTOCOL, REQ_DENY} {
		r[passThroughFtd].rules[ruleType] = r[offloadFtd].rules[ruleType]
		for _, rule := range r[passThroughFtd].rules[ruleType] {
			ruleID := rule.GetID()
			delete(r[offloadFtd].status, ruleID)
			r[passThroughFtd].status[ruleID] = CREATED
		}
	}
}

func (r Rules) Clean(frontends ...string) {
	for _, frontend := range frontends {
		if ftRules, ok := r[frontend]; ok {
			for id := range ftRules.status {
				ftRules.status[id] = TO_DELETE
			}
		}
	}
}

func (r Rules) Refresh(client api.HAProxyClient) (reload bool) {
	for feName, ftRules := range r {
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
			ruleSet := ftRules.rules[ruleType]
			for i := len(ruleSet) - 1; i >= 0; i-- {
				ruleID := ruleSet[i].GetID()
				if ftRules.status[ruleID] == TO_DELETE {
					delete(ftRules.status, ruleID)
					ftRules.rules[ruleType] = append(ruleSet[:i], ruleSet[i+1:]...)
					continue
				}
				err := ruleSet[i].Create(client, &fe)
				logger.Error(err)
				if err == nil && ftRules.status[ruleID] == TO_CREATE {
					reload = true
				}
			}
		}
	}
	return reload
}
