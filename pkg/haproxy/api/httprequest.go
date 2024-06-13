package api

import "github.com/haproxytech/client-native/v5/models"

func (c *clientNative) HTTPRequestRulesGet(parentType, parentName string) (models.HTTPRequestRules, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	_, httpRequests, err := configuration.GetHTTPRequestRules(parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return httpRequests, nil
}

func (c *clientNative) HTTPRequestRuleGet(id int64, parentType, parentName string) (*models.HTTPRequestRule, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	_, httpRequest, err := configuration.GetHTTPRequestRule(id, parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return httpRequest, nil
}

func (c *clientNative) HTTPRequestRuleDelete(id int64, parentType string, parentName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.DeleteHTTPRequestRule(id, parentType, parentName, c.activeTransaction, 0)
}

func (c *clientNative) HTTPRequestRuleDeleteAll(parentType string, parentName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	_, httpRequests, errGet := configuration.GetHTTPRequestRules(parentType, parentName, c.activeTransaction)
	if errGet != nil {
		return errGet
	}

	for range httpRequests {
		errDelete := configuration.DeleteHTTPRequestRule(0, parentType, parentName, c.activeTransaction, 0)
		if errDelete != nil {
			return errDelete
		}
	}
	return nil
}

func (c *clientNative) HTTPRequestRuleCreate(parentType string, parentName string, data *models.HTTPRequestRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.CreateHTTPRequestRule(parentType, parentName, data, c.activeTransaction, 0)
}

func (c *clientNative) HTTPRequestRuleEdit(id int64, parentType string, parentName string, data *models.HTTPRequestRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.EditHTTPRequestRule(id, parentType, parentName, data, c.activeTransaction, 0)
}
