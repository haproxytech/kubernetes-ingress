package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
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
		return backend.HTTPRequestRuleList, nil
	}

	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get http requests rules for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		return frontend.HTTPRequestRuleList, nil
	}

	_, httpRequests, err := configuration.GetHTTPRequestRules(parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return httpRequests, nil
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
		backend.HTTPRequestRuleList = nil
		c.backends[parentName] = backend
		return nil
	}

	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't delete http requests rules for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.HTTPRequestRuleList = nil
		c.frontends[parentName] = frontend
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

func (c *clientNative) HTTPRequestRuleCreate(id int64, parentType string, parentName string, data *models.HTTPRequestRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't create http request rule for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPRequestRuleList = append(backend.HTTPRequestRuleList, data)
		c.backends[parentName] = backend
		return nil
	}
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't create http request rule for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.HTTPRequestRuleList = append(frontend.HTTPRequestRuleList, data)
		c.frontends[parentName] = frontend
		return nil
	}
	return configuration.CreateHTTPRequestRule(id, parentType, parentName, data, c.activeTransaction, 0)
}

func (c *clientNative) HTTPRequestRulesReplace(parentType, parentName string, rules models.HTTPRequestRules) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}

	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't replace http request rule for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPRequestRuleList = rules
		c.backends[parentName] = backend
		return nil
	}

	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't replace http request rule for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.HTTPRequestRuleList = rules
		c.frontends[parentName] = frontend
		return nil
	}

	return configuration.ReplaceHTTPRequestRules(parentType, parentName, rules, c.activeTransaction, 0)
}
