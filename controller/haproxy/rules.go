package haproxy

import (
	"encoding/json"
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type Rule interface {
	Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error
	GetType() RuleType
}

// IngressACLVar is the HAProxy variable
// to be matched against the ruleID
var IngressACLVar = "txn.match"

// Order matters !
// Rules will be evaluated by HAProxy in the defined order.
type RuleType int

//nolint
const (
	REQ_ACCEPT_CONTENT RuleType = iota
	REQ_INSPECT_DELAY
	REQ_PROXY_PROTOCOL
	REQ_SET_VAR
	REQ_SET_SRC
	REQ_DENY
	REQ_TRACK
	REQ_AUTH
	REQ_RATELIMIT
	REQ_CAPTURE
	REQ_REQUEST_REDIRECT
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
	// exclusive states
	CREATED   RuleStatus = 0
	TO_CREATE RuleStatus = 1
	TO_DELETE RuleStatus = 2
	// non exclusive states
	INGRESS RuleStatus = 4
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

func (r Rules) AddRule(rule Rule, ingressName string, frontends ...string) error {
	if rule == nil || len(frontends) == 0 {
		return fmt.Errorf("invalid params")
	}
	id := getID(rule)
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
	}
	if ingressName != "" {
		for _, frontend := range frontends {
			r.frontendRules[frontend].status[id] |= INGRESS
		}
		r.ingressRuleIDs[ingressName] = append(r.ingressRuleIDs[ingressName], id)
	}
	return nil
}

func (r Rules) DeleteFrontend(frontend string) {
	delete(r.frontendRules, frontend)
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

func (r Rules) GetIngressRuleIDs(ingress string) (ruleIDs []RuleID) {
	ids := r.ingressRuleIDs[ingress]
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
				ingressACL := ""
				id := getID(ruleSet[i])
				if ftRules.status[id] == TO_DELETE {
					delete(ftRules.status, id)
					ruleSet = append(ruleSet[:i], ruleSet[i+1:]...)
					continue
				}
				if ftRules.status[id]&INGRESS != 0 {
					ingressACL = fmt.Sprintf("{ var(%s) -m dom %d }", IngressACLVar, id)
				}
				err := ruleSet[i].Create(client, &fe, ingressACL)
				logger.Error(err)
				if err == nil && ftRules.status[id]&TO_CREATE != 0 {
					reload = true
				}
			}
			ftRules.rules[ruleType] = ruleSet
		}
	}
	return reload
}

func getID(rule Rule) RuleID {
	b, _ := json.Marshal(rule)
	b = append(b, byte(rule.GetType()))
	return RuleID(utils.Hash(b))
}
