package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) HTTPErrorRulesGet(parentType, parentName string) (models.HTTPErrorRules, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get http error rules for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		return backend.HTTPErrorRuleList, nil
	}
	_, rules, err := configuration.GetHTTPErrorRules(parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (c *clientNative) HTTPErrorRuleDeleteAll(parentType, parentName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't delete http error rules for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPErrorRuleList = nil
		c.backends[parentName] = backend
		return nil
	}
	_, rules, errGet := configuration.GetHTTPErrorRules(parentType, parentName, c.activeTransaction)
	if errGet != nil {
		return errGet
	}
	for range rules {
		errDelete := configuration.DeleteHTTPErrorRule(0, parentType, parentName, c.activeTransaction, 0)
		if errDelete != nil {
			return errDelete
		}
	}
	return nil
}

func (c *clientNative) HTTPErrorRuleCreate(id int64, parentType, parentName string, data *models.HTTPErrorRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't create http error rule for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPErrorRuleList = append(backend.HTTPErrorRuleList, data)
		c.backends[parentName] = backend
		return nil
	}
	return configuration.CreateHTTPErrorRule(id, parentType, parentName, data, c.activeTransaction, 0)
}

func (c *clientNative) HTTPErrorRulesReplace(parentType, parentName string, rules models.HTTPErrorRules) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't replace http error rules for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPErrorRuleList = rules
		c.backends[parentName] = backend
		return nil
	}
	return configuration.ReplaceHTTPErrorRules(parentType, parentName, rules, c.activeTransaction, 0)
}
