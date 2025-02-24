package api

import "github.com/haproxytech/client-native/v6/models"

func (c *clientNative) TCPRequestRuleCreate(id int64, parentType, parentName string, rule models.TCPRequestRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.CreateTCPRequestRule(id, parentType, parentName, &rule, c.activeTransaction, 0)
}

func (c *clientNative) TCPRequestRuleDeleteAll(parentType, parentName string) (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return
	}
	_, rules, err := configuration.GetTCPRequestRules(parentType, parentName, c.activeTransaction)
	if err != nil {
		return
	}
	for range rules {
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

func (c *clientNative) TCPRequestRulesReplace(parentType, parentName string, rules models.TCPRequestRules) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}

	err = configuration.ReplaceTCPRequestRules(parentType, parentName, rules, c.activeTransaction, 0)
	if err != nil {
		return err
	}
	return nil
}
