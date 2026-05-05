package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) TCPResponseRulesGet(parentType, parentName string) (models.TCPResponseRules, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get tcp response rules for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		return backend.TCPResponseRuleList, nil
	}
	_, rules, err := configuration.GetTCPResponseRules(parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (c *clientNative) TCPResponseRuleDeleteAll(parentType, parentName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't delete tcp response rules for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.TCPResponseRuleList = nil
		c.backends[parentName] = backend
		return nil
	}
	_, rules, errGet := configuration.GetTCPResponseRules(parentType, parentName, c.activeTransaction)
	if errGet != nil {
		return errGet
	}
	for range rules {
		errDelete := configuration.DeleteTCPResponseRule(0, parentType, parentName, c.activeTransaction, 0)
		if errDelete != nil {
			return errDelete
		}
	}
	return nil
}

func (c *clientNative) TCPResponseRuleCreate(id int64, parentType, parentName string, data *models.TCPResponseRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't create tcp response rule for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.TCPResponseRuleList = append(backend.TCPResponseRuleList, data)
		c.backends[parentName] = backend
		return nil
	}
	return configuration.CreateTCPResponseRule(id, parentType, parentName, data, c.activeTransaction, 0)
}

func (c *clientNative) TCPResponseRulesReplace(parentType, parentName string, rules models.TCPResponseRules) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't replace tcp response rules for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.TCPResponseRuleList = rules
		c.backends[parentName] = backend
		return nil
	}
	return configuration.ReplaceTCPResponseRules(parentType, parentName, rules, c.activeTransaction, 0)
}
