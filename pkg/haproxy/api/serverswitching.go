package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) ServerSwitchingRulesGet(backendName string) (models.ServerSwitchingRules, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	backend, exists := c.backends[backendName]
	if exists {
		return backend.ServerSwitchingRuleList, nil
	}
	_, rules, err := configuration.GetServerSwitchingRules(backendName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (c *clientNative) ServerSwitchingRuleDeleteAll(backendName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if backend, exists := c.backends[backendName]; exists {
		backend.ServerSwitchingRuleList = nil
		c.backends[backendName] = backend
		return nil
	}
	_, rules, errGet := configuration.GetServerSwitchingRules(backendName, c.activeTransaction)
	if errGet != nil {
		return errGet
	}
	for range rules {
		errDelete := configuration.DeleteServerSwitchingRule(0, backendName, c.activeTransaction, 0)
		if errDelete != nil {
			return errDelete
		}
	}
	return nil
}

func (c *clientNative) ServerSwitchingRuleCreate(id int64, backendName string, data *models.ServerSwitchingRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if backend, exists := c.backends[backendName]; exists {
		backend.ServerSwitchingRuleList = append(backend.ServerSwitchingRuleList, data)
		c.backends[backendName] = backend
		return nil
	}
	return configuration.CreateServerSwitchingRule(id, backendName, data, c.activeTransaction, 0)
}

func (c *clientNative) ServerSwitchingRulesReplace(backendName string, rules models.ServerSwitchingRules) error {
	if backendName == "" {
		return fmt.Errorf("can't replace server-switching rules: backend has no name : %w", ErrNotFound)
	}
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if backend, exists := c.backends[backendName]; exists {
		backend.ServerSwitchingRuleList = rules
		c.backends[backendName] = backend
		return nil
	}
	return configuration.ReplaceServerSwitchingRules(backendName, rules, c.activeTransaction, 0)
}
