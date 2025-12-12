package api

import (
	"errors"
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) BackendSwitchingRuleCreate(id int64, frontendName string, rule models.BackendSwitchingRule) error {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return fmt.Errorf("frontend %s not found", frontendName)
	}
	if id != 0 && id >= int64(len(frontend.BackendSwitchingRuleList)-1) {
		return errors.New("can't add rule to the last position in the list")
	}
	if frontend.BackendSwitchingRuleList == nil {
		frontend.BackendSwitchingRuleList = models.BackendSwitchingRules{}
	}

	frontend.BackendSwitchingRuleList = append(frontend.BackendSwitchingRuleList, &models.BackendSwitchingRule{})
	copy(frontend.BackendSwitchingRuleList[id+1:], frontend.BackendSwitchingRuleList[id:])
	frontend.BackendSwitchingRuleList[id] = &rule
	return nil
}

func (c *clientNative) BackendSwitchingRuleDeleteAll(frontendName string) (err error) {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return fmt.Errorf("frontend %s not found", frontendName)
	}
	frontend.BackendSwitchingRuleList = nil
	return nil
}

func (c *clientNative) BackendSwitchingRulesGet(frontendName string) (models.BackendSwitchingRules, error) {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return nil, fmt.Errorf("frontend %s not found", frontendName)
	}
	return frontend.BackendSwitchingRuleList, nil
}

func (c *clientNative) BackendSwitchingRulesReplace(frontendName string, rules models.BackendSwitchingRules) error {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return fmt.Errorf("frontend %s not found", frontendName)
	}
	frontend.BackendSwitchingRuleList = rules
	return nil
}
