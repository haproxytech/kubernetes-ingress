package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) HTTPResponseRulesGet(parentType, parentName string) (models.HTTPResponseRules, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get http requests rules for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		return backend.HTTPResponseRuleList, nil
	}

	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get http requests rules for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		return frontend.HTTPResponseRuleList, nil
	}

	_, httpResponses, err := configuration.GetHTTPResponseRules(parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return httpResponses, nil
}

func (c *clientNative) HTTPResponseRuleDeleteAll(parentType string, parentName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't delete http requests rules for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPResponseRuleList = nil
		c.backends[parentName] = backend
		return nil
	}

	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't delete http requests rules for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.HTTPResponseRuleList = nil
		c.frontends[parentName] = frontend
		return nil
	}

	_, httpResponses, errGet := configuration.GetHTTPResponseRules(parentType, parentName, c.activeTransaction)
	if errGet != nil {
		return errGet
	}

	for range httpResponses {
		errDelete := configuration.DeleteHTTPResponseRule(0, parentType, parentName, c.activeTransaction, 0)
		if errDelete != nil {
			return errDelete
		}
	}
	return nil
}

func (c *clientNative) HTTPResponseRuleCreate(id int64, parentType string, parentName string, data *models.HTTPResponseRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't create http request rule for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPResponseRuleList = append(backend.HTTPResponseRuleList, data)
		c.backends[parentName] = backend
		return nil
	}
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't create http request rule for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.HTTPResponseRuleList = append(frontend.HTTPResponseRuleList, data)
		c.frontends[parentName] = frontend
		return nil
	}
	return configuration.CreateHTTPResponseRule(id, parentType, parentName, data, c.activeTransaction, 0)
}

func (c *clientNative) HTTPResponseRulesReplace(parentType, parentName string, rules models.HTTPResponseRules) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}

	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't replace http request rule for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPResponseRuleList = rules
		c.backends[parentName] = backend
		return nil
	}

	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't replace http request rule for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.HTTPResponseRuleList = rules
		c.frontends[parentName] = frontend
		return nil
	}

	return configuration.ReplaceHTTPResponseRules(parentType, parentName, rules, c.activeTransaction, 0)
}
