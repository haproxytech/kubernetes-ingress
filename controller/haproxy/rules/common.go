package rules

import (
	"encoding/json"
	"fmt"
	"hash/fnv"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
)

var PatternVar = "txn.match"

func matchRuleID(rule interface{}, ruleID uint32) {
	switch r := rule.(type) {
	case *models.TCPRequestRule:
		r.Cond = "if"
		r.CondTest = fmt.Sprintf("{ var(%s) -m dom %d } %s", PatternVar, ruleID, r.CondTest)
	case *models.HTTPRequestRule:
		r.Cond = "if"
		r.CondTest = fmt.Sprintf("{ var(%s) -m dom %d } %s", PatternVar, ruleID, r.CondTest)
	}

}
func hashRule(rule haproxy.Rule) uint32 {
	b, _ := json.Marshal(rule)
	b = append(b, byte(rule.GetType()))
	h := fnv.New32a()
	_, _ = h.Write(b)
	return h.Sum32()
}
