package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v5/models"
)

func (c *clientNative) HTTPRequestRulesGet(parentType, parentName string) (models.HTTPRequestRules, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get http requests rules for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		return backend.HTTPRequestsRules, nil
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
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't delete http requests rules for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPRequestsRules = nil
		c.backends[parentName] = backend
		return nil
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
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't create http request rule for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPRequestsRules = append(backend.HTTPRequestsRules, data)
		c.backends[parentName] = backend
		return nil
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
