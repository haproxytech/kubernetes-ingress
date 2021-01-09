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

// RuleStatus describing Rule creation
type RuleStatus int

// RuleID uniquely identify a HAProxy Rule
type RuleID uint32

//nolint
const (
	TO_DELETE RuleStatus = iota
	TO_CREATE
	CREATED
)

type Rules struct {
	frontendRules  map[string]*ruleset
	ingressRuleIDs map[string][]RuleID
}

type ruleset struct {
	// rules holds a map of HAProxy rules
	// grouped by rule types
	rules map[RuleType][]Rule
	// status holds a map of RuleIDs and
	// the corresponding ruleStatus
	status map[RuleID]RuleStatus
}

// module logger
var logger = utils.GetLogger()

func NewRules() *Rules {
	return &Rules{
		// frontend rules
		frontendRules: make(map[string]*ruleset),
		// ruleIDs grouped by ingressName
		ingressRuleIDs: make(map[string][]RuleID),
	}
}

func (r Rules) AddRule(rule Rule, ingressName *string, frontends ...string) error {
	if rule == nil || len(frontends) == 0 {
		return fmt.Errorf("invalid params")
	}
	id := RuleID(rule.GetID())
	ruleType := rule.GetType()
	for _, frontend := range frontends {
		ftRules, ok := r.frontendRules[frontend]
		// Create frontend ruleSet
		if !ok {
			ftRules = &ruleset{
				rules:  make(map[RuleType][]Rule),
				status: make(map[RuleID]RuleStatus),
			}
			r.frontendRules[frontend] = ftRules
		}
		// Update frontend ruleSet
		if _, ok := ftRules.status[id]; ok {
			// Rule already created
			ftRules.status[id] = CREATED
		} else {
			// Rule to create at next refresh
			ftRules.rules[ruleType] = append(ftRules.rules[ruleType], rule)
			ftRules.status[id] = TO_CREATE
		}
		if ingressName != nil {
			r.ingressRuleIDs[*ingressName] = append(r.ingressRuleIDs[*ingressName], id)
		}
	}
	return nil
}

func (r Rules) EnableSSLPassThrough(passThroughFtd, offloadFtd string) {
	if _, ok := r.frontendRules[offloadFtd]; !ok {
		return
	}
	if _, ok := r.frontendRules[passThroughFtd]; !ok {
		r.frontendRules[passThroughFtd] = &ruleset{
			rules:  make(map[RuleType][]Rule),
			status: make(map[RuleID]RuleStatus),
		}
	}
	// Move some layer 4 rules from sslOffloading Frontend to sslPassthrough Frontend
	for _, ruleType := range []RuleType{REQ_PROXY_PROTOCOL, REQ_DENY} {
		r.frontendRules[passThroughFtd].rules[ruleType] = r.frontendRules[offloadFtd].rules[ruleType]
		for _, rule := range r.frontendRules[passThroughFtd].rules[ruleType] {
			id := RuleID(rule.GetID())
			delete(r.frontendRules[offloadFtd].status, id)
			r.frontendRules[passThroughFtd].status[id] = CREATED
		}
	}
}

func (r Rules) Clean(frontends ...string) {
	for _, frontend := range frontends {
		if ftRules, ok := r.frontendRules[frontend]; ok {
			for id := range ftRules.status {
				ftRules.status[id] = TO_DELETE
			}
		}
	}
	for ingress := range r.ingressRuleIDs {
		r.ingressRuleIDs[ingress] = r.ingressRuleIDs[ingress][:0]
	}
}

func (r Rules) PopIngressRuleIDs(ingress string) (ruleIDs []RuleID) {
	ids := r.ingressRuleIDs[ingress]
	r.ingressRuleIDs[ingress] = r.ingressRuleIDs[ingress][:0]
	return ids
}

func (r Rules) Refresh(client api.HAProxyClient) (reload bool) {
	for feName, ftRules := range r.frontendRules {
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
				id := RuleID(ruleSet[i].GetID())
				if ftRules.status[id] == TO_DELETE {
					delete(ftRules.status, id)
					ftRules.rules[ruleType] = append(ruleSet[:i], ruleSet[i+1:]...)
					continue
				}
				err := ruleSet[i].Create(client, &fe)
				logger.Error(err)
				if err == nil && ftRules.status[id] == TO_CREATE {
					reload = true
				}
			}
		}
	}
	return reload
}
