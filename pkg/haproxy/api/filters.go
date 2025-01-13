package api

import "github.com/haproxytech/client-native/v6/models"

func (c *clientNative) FilterCreate(id int64, parentType, parentName string, rule models.Filter) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.CreateFilter(id, parentType, parentName, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FilterDeleteAll(parentType, parentName string) (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return
	}
	_, rules, err := configuration.GetFilters(parentType, parentName, c.activeTransaction)
	if err != nil {
		return
	}
	for range len(rules) {
		if err = configuration.DeleteFilter(0, parentType, parentName, c.activeTransaction, 0); err != nil {
			break
		}
	}
	return
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

func (c *clientNative) FiltersReplace(parentType, parentName string, rules models.Filters) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}

	err = configuration.ReplaceFilters(parentType, parentName, rules, c.activeTransaction, 0)
	if err != nil {
		return err
	}
	return nil
}
