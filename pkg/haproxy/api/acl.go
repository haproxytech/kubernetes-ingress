package api

import "github.com/haproxytech/client-native/v5/models"

func (c *clientNative) ACLsGet(parentType, parentName string, aclName ...string) (models.Acls, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	_, acls, err := configuration.GetACLs(parentType, parentName, c.activeTransaction, aclName...)
	if err != nil {
		return nil, err
	}
	return acls, nil
}

func (c *clientNative) ACLGet(id int64, parentType, parentName string) (*models.ACL, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	_, acl, err := configuration.GetACL(id, parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return acl, nil
}

func (c *clientNative) ACLDelete(id int64, parentType string, parentName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.DeleteACL(id, parentType, parentName, c.activeTransaction, 0)
}

func (c *clientNative) ACLDeleteAll(parentType string, parentName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
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

func (c *clientNative) ACLCreate(parentType string, parentName string, data *models.ACL) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.CreateACL(parentType, parentName, data, c.activeTransaction, 0)
}

func (c *clientNative) ACLEdit(id int64, parentType string, parentName string, data *models.ACL) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.EditACL(id, parentType, parentName, data, c.activeTransaction, 0)
}
