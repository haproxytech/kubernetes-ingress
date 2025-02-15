package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) ACLsGet(parentType, parentName string, aclName ...string) (models.Acls, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}

	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get acls for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		return backend.ACLList, nil
	}

	_, acls, err := configuration.GetACLs(parentType, parentName, c.activeTransaction, aclName...)
	if err != nil {
		return nil, err
	}
	return acls, nil
}

func (c *clientNative) ACLDeleteAll(parentType string, parentName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}

	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't delete acls for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.ACLList = nil
		c.backends[parentName] = backend
		return nil
	}

	_, acls, errGet := configuration.GetACLs(parentType, parentName, c.activeTransaction)
	if errGet != nil {
		return errGet
	}

	for range acls {
		errDelete := configuration.DeleteACL(0, parentType, parentName, c.activeTransaction, 0)
		if errDelete != nil {
			return errDelete
		}
	}
	return nil
}

func (c *clientNative) ACLCreate(id int64, parentType string, parentName string, data *models.ACL) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't create acl for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.ACLList = append(backend.ACLList, data)
		c.backends[parentName] = backend
		return nil
	}
	return configuration.CreateACL(id, parentType, parentName, data, c.activeTransaction, 0)
}

func (c *clientNative) ACLsReplace(parentType, parentName string, rules models.Acls) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if parentType == "backend" {
		backend, exists := c.backends[parentName]
		if !exists {
			return fmt.Errorf("can't replace acl for unexisting backend %s : %w", parentName, ErrNotFound)
		}
		backend.ACLList = rules
		c.backends[parentName] = backend
		return nil
	}

	return configuration.ReplaceAcls(parentType, parentName, rules, c.activeTransaction, 0)
}
