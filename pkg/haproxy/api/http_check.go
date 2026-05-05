package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) HTTPChecksGet(parentType, parentName string) (models.HTTPChecks, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get http checks for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		return backend.HTTPCheckList, nil
	}
	_, rules, err := configuration.GetHTTPChecks(parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (c *clientNative) HTTPCheckDeleteAll(parentType, parentName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't delete http checks for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPCheckList = nil
		c.backends[parentName] = backend
		return nil
	}
	_, rules, errGet := configuration.GetHTTPChecks(parentType, parentName, c.activeTransaction)
	if errGet != nil {
		return errGet
	}
	for range rules {
		errDelete := configuration.DeleteHTTPCheck(0, parentType, parentName, c.activeTransaction, 0)
		if errDelete != nil {
			return errDelete
		}
	}
	return nil
}

func (c *clientNative) HTTPCheckCreate(id int64, parentType, parentName string, data *models.HTTPCheck) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't create http check for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPCheckList = append(backend.HTTPCheckList, data)
		c.backends[parentName] = backend
		return nil
	}
	return configuration.CreateHTTPCheck(id, parentType, parentName, data, c.activeTransaction, 0)
}

func (c *clientNative) HTTPChecksReplace(parentType, parentName string, rules models.HTTPChecks) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't replace http checks for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.HTTPCheckList = rules
		c.backends[parentName] = backend
		return nil
	}
	return configuration.ReplaceHTTPChecks(parentType, parentName, rules, c.activeTransaction, 0)
}
