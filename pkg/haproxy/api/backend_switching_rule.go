package api

import "github.com/haproxytech/client-native/v6/models"

func (c *clientNative) BackendSwitchingRuleCreate(id int64, frontend string, rule models.BackendSwitchingRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.CreateBackendSwitchingRule(id, frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) BackendSwitchingRuleDeleteAll(frontend string) (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return
	}
	_, switchingRules, err := configuration.GetBackendSwitchingRules(frontend, c.activeTransaction)
	if err != nil {
		return
	}
	for range len(switchingRules) {
		if err = configuration.DeleteBackendSwitchingRule(0, frontend, c.activeTransaction, 0); err != nil {
			break
		}
	}
	return
}

func (c *clientNative) BackendSwitchingRulesGet(frontend string) (models.BackendSwitchingRules, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}

	_, bsRules, err := configuration.GetBackendSwitchingRules(frontend, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return bsRules, nil
}

func (c *clientNative) BackendSwitchingRulesReplace(frontend string, rules models.BackendSwitchingRules) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}

	err = configuration.ReplaceBackendSwitchingRules(frontend, rules, c.activeTransaction, 0)
	if err != nil {
		return err
	}
	return nil
}
