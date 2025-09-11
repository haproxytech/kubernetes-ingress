package api

import "github.com/haproxytech/client-native/v5/models"

func (c *clientNative) FilterCreate(parentType, parentName string, rule models.Filter) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.CreateFilter(parentType, parentName, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FilterDeleteAll(parentType, parentName string) (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	_, rules, err := configuration.GetFilters(parentType, parentName, c.activeTransaction)
	if err != nil {
		return err
	}
	for range rules {
		if err = configuration.DeleteFilter(0, parentType, parentName, c.activeTransaction, 0); err != nil {
			break
		}
	}
	return err
}

func (c *clientNative) FiltersGet(parentType, parentName string) (models.Filters, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}

	_, rules, err := configuration.GetFilters(parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return rules, nil
}
