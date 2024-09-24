package api

import "github.com/haproxytech/client-native/v5/models"

func (c *clientNative) TCPRequestRuleCreate(parentType, parentName string, rule models.TCPRequestRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.CreateTCPRequestRule(parentType, parentName, &rule, c.activeTransaction, 0)
}

func (c *clientNative) TCPRequestRuleDeleteAll(parentType, parentName string) (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return
	}
	c.activeTransactionHasChanges = true
	_, rules, err := configuration.GetTCPRequestRules(parentType, parentName, c.activeTransaction)
	if err != nil {
		return
	}
	for range len(rules) {
		if err = configuration.DeleteTCPRequestRule(0, parentType, parentName, c.activeTransaction, 0); err != nil {
			break
		}
	}
	return
}

func (c *clientNative) TCPRequestRulesGet(parentType, parentName string) (models.TCPRequestRules, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}

	_, rules, err := configuration.GetTCPRequestRules(parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return rules, nil
}
