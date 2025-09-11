package api

import "github.com/haproxytech/client-native/v5/models"

func (c *clientNative) BackendSwitchingRuleCreate(frontend string, rule models.BackendSwitchingRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.CreateBackendSwitchingRule(frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) BackendSwitchingRuleDeleteAll(frontend string) (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	_, switchingRules, err := configuration.GetBackendSwitchingRules(frontend, c.activeTransaction)
	if err != nil {
		return err
	}
	for range switchingRules {
		if err = configuration.DeleteBackendSwitchingRule(0, frontend, c.activeTransaction, 0); err != nil {
			break
		}
	}
	return err
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
