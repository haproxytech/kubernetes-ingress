package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) HTTPAfterResponseRulesGet(parentType, parentName string) (models.HTTPAfterResponseRules, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get http after response rules for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		return backend.HTTPAfterResponseRuleList, nil
	}

	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get http after response rules for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		return frontend.HTTPAfterResponseRuleList, nil
	}

	_, rules, err := configuration.GetHTTPAfterResponseRules(parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (c *clientNative) HTTPAfterResponseRuleDeleteAll(parentType string, parentName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't delete http after response rules for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPAfterResponseRuleList = nil
		c.backends[parentName] = backend
		return nil
	}

	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't delete http after response rules for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.HTTPAfterResponseRuleList = nil
		c.frontends[parentName] = frontend
		return nil
	}

	_, rules, errGet := configuration.GetHTTPAfterResponseRules(parentType, parentName, c.activeTransaction)
	if errGet != nil {
		return errGet
	}

	for range rules {
		errDelete := configuration.DeleteHTTPAfterResponseRule(0, parentType, parentName, c.activeTransaction, 0)
		if errDelete != nil {
			return errDelete
		}
	}
	return nil
}

func (c *clientNative) HTTPAfterResponseRuleCreate(id int64, parentType string, parentName string, data *models.HTTPAfterResponseRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't create http after response rule for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPAfterResponseRuleList = append(backend.HTTPAfterResponseRuleList, data)
		c.backends[parentName] = backend
		return nil
	}
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't create http after response rule for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.HTTPAfterResponseRuleList = append(frontend.HTTPAfterResponseRuleList, data)
		c.frontends[parentName] = frontend
		return nil
	}
	return configuration.CreateHTTPAfterResponseRule(id, parentType, parentName, data, c.activeTransaction, 0)
}

func (c *clientNative) HTTPAfterResponseRulesReplace(parentType, parentName string, rules models.HTTPAfterResponseRules) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}

	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't replace http after response rule for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPAfterResponseRuleList = rules
		c.backends[parentName] = backend
		return nil
	}

	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't replace http after response rule for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.HTTPAfterResponseRuleList = rules
		c.frontends[parentName] = frontend
		return nil
	}

	return configuration.ReplaceHTTPAfterResponseRules(parentType, parentName, rules, c.activeTransaction, 0)
}
