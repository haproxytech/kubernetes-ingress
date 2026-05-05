package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) TCPChecksGet(parentType, parentName string) (models.TCPChecks, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get tcp checks for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		return backend.TCPCheckRuleList, nil
	}
	_, rules, err := configuration.GetTCPChecks(parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (c *clientNative) TCPCheckDeleteAll(parentType, parentName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't delete tcp checks for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.TCPCheckRuleList = nil
		c.backends[parentName] = backend
		return nil
	}
	_, rules, errGet := configuration.GetTCPChecks(parentType, parentName, c.activeTransaction)
	if errGet != nil {
		return errGet
	}
	for range rules {
		errDelete := configuration.DeleteTCPCheck(0, parentType, parentName, c.activeTransaction, 0)
		if errDelete != nil {
			return errDelete
		}
	}
	return nil
}

func (c *clientNative) TCPCheckCreate(id int64, parentType, parentName string, data *models.TCPCheck) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't create tcp check for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.TCPCheckRuleList = append(backend.TCPCheckRuleList, data)
		c.backends[parentName] = backend
		return nil
	}
	return configuration.CreateTCPCheck(id, parentType, parentName, data, c.activeTransaction, 0)
}

func (c *clientNative) TCPChecksReplace(parentType, parentName string, rules models.TCPChecks) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't replace tcp checks for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.TCPCheckRuleList = rules
		c.backends[parentName] = backend
		return nil
	}
	return configuration.ReplaceTCPChecks(parentType, parentName, rules, c.activeTransaction, 0)
}
