package rules

import (
	"encoding/json"
	"fmt"

	"github.com/haproxytech/client-native/v3/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// module logger
var logger = utils.GetLogger()

// HTTPACLVar used to match against RuleID in haproxy http frontend
var HTTPACLVar = "txn.path_match"

// TCPACLVar used to match against RuleID in haproxy ssl frontend
var TCPACLVar = "txn.sni_match"

type Rules interface {
	AddRule(frontend string, rule Rule, ingressRule bool) error
	DeleteFTRules(frontend string)
	CleanRules()
	RefreshRules(client api.HAProxyClient) (reload bool)
}

type Rule interface {
	Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error
	GetType() Type
}

type List []Rule

// RuleID uniquely identify a HAProxy Rule
type RuleID string

type SectionRules map[string]*ruleset

type ruleset struct {
	// rules is a map of HAProxy rules
	// grouped by rule types
	rules map[Type]List
	// meta is a map of RuleIDs and
	// the corresponding ruleInfo
	meta map[RuleID]*ruleInfo
}

// ruleInfo holds information about a HAProxy rule
type ruleInfo struct {
	state   ruleState
	ingress bool
}

// ruleState describes Rule creation
type ruleState int

//nolint: golint,stylecheck
const (
	CREATED   ruleState = 0
	TO_CREATE ruleState = 1
	TO_DELETE ruleState = 2
)

func New() *SectionRules {
	return &SectionRules{}
}

func (rules *List) Add(rule Rule) {
	*rules = append(*rules, rule)
}

func (r SectionRules) AddRule(frontend string, rule Rule, ingressRule bool) error {
	if rule == nil || frontend == "" {
		return fmt.Errorf("invalid params")
	}
	// Create frontend ruleSet
	ftRuleSet, ok := r[frontend]
	if !ok {
		ftRuleSet = &ruleset{
			rules: make(map[Type]List),
			meta:  make(map[RuleID]*ruleInfo),
		}
		r[frontend] = ftRuleSet
	}
	// Update frontend ruleSet
	ruleType := rule.GetType()
	ruleID := GetID(rule)
	_, ok = ftRuleSet.meta[ruleID]
	if ok {
		// rule already created
		ftRuleSet.meta[ruleID].state = CREATED
	} else {
		// rule to be created at next refresh
		ftRuleSet.rules[ruleType] = append(ftRuleSet.rules[ruleType], rule)
		ftRuleSet.meta[ruleID] = &ruleInfo{state: TO_CREATE}
	}
	if ingressRule {
		ftRuleSet.meta[ruleID].ingress = true
	}
	return nil
}

func (r SectionRules) DeleteFTRules(frontend string) {
	delete(r, frontend)
}

func (r SectionRules) CleanRules() {
	for frontend := range r {
		if ftRuleSet, ok := r[frontend]; ok {
			for id := range ftRuleSet.meta {
				ftRuleSet.meta[id].state = TO_DELETE
			}
		}
	}
}

func (r SectionRules) RefreshRules(client api.HAProxyClient) (reload bool) {
	logger.Error(client.UserListDeleteAll())
	for feName := range r {
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
			for i := len(r[feName].rules[ruleType]) - 1; i >= 0; i-- {
				reload = r.refreshRule(client, ruleType, i, &fe) || reload
			}
		}
	}
	return reload
}

func (r SectionRules) refreshRule(client api.HAProxyClient, ruleType Type, i int, frontend *models.Frontend) (reload bool) {
	aclVar := HTTPACLVar
	if frontend.Mode == "tcp" {
		aclVar = TCPACLVar
	}
	frontendRuleSet := r[frontend.Name]
	rules := frontendRuleSet.rules[ruleType]
	id := GetID(rules[i])
	// Delete HAProxy Rule
	if frontendRuleSet.meta[id].state == TO_DELETE {
		delete(frontendRuleSet.meta, id)
		frontendRuleSet.rules[ruleType] = append(rules[:i], rules[i+1:]...)
		logger.Debugf("HAProxy rule '%s' deleted", constLookup[ruleType])
		reload = true
		return
	}
	// Create HAProxy Rule
	ingressACL := ""
	if frontendRuleSet.meta[id].ingress {
		ingressACL = fmt.Sprintf("{ var(%s) -m dom %s }", aclVar, id)
	}
	err := rules[i].Create(client, frontend, ingressACL)
	if err != nil {
		logger.Errorf("failed to create a %s rule: %s", constLookup[ruleType], err)
		return
	}
	if frontendRuleSet.meta[id].state == TO_CREATE {
		reload = true
		logger.Debugf("New HAProxy rule '%s' created, reload required", constLookup[ruleType])
	}
	return
}

func GetID(rule Rule) RuleID {
	b, _ := json.Marshal(rule)
	b = append(b, byte(rule.GetType()))
	return RuleID(utils.Hash(b))
}
