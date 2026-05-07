package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) StickRulesGet(backendName string) (models.StickRules, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	if backend, exists := c.backends[backendName]; exists {
		return backend.StickRuleList, nil
	}
	_, rules, err := configuration.GetStickRules(backendName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (c *clientNative) StickRuleDeleteAll(backendName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if backend, exists := c.backends[backendName]; exists {
		backend.StickRuleList = nil
		c.backends[backendName] = backend
		return nil
	}
	_, rules, errGet := configuration.GetStickRules(backendName, c.activeTransaction)
	if errGet != nil {
		return errGet
	}
	for range rules {
		errDelete := configuration.DeleteStickRule(0, backendName, c.activeTransaction, 0)
		if errDelete != nil {
			return errDelete
		}
	}
	return nil
}

func (c *clientNative) StickRuleCreate(id int64, backendName string, data *models.StickRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if backend, exists := c.backends[backendName]; exists {
		backend.StickRuleList = append(backend.StickRuleList, data)
		c.backends[backendName] = backend
		return nil
	}
	return configuration.CreateStickRule(id, backendName, data, c.activeTransaction, 0)
}

func (c *clientNative) StickRulesReplace(backendName string, rules models.StickRules) error {
	if backendName == "" {
		return fmt.Errorf("can't replace stick rules: backend has no name : %w", ErrNotFound)
	}
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if backend, exists := c.backends[backendName]; exists {
		backend.StickRuleList = rules
		c.backends[backendName] = backend
		return nil
	}
	return configuration.ReplaceStickRules(backendName, rules, c.activeTransaction, 0)
}
