package api

import (
	"errors"
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) FilterCreate(id int64, parentType, parentName string, rule models.Filter) error {
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't create filter for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		if id != 0 && id >= int64(len(frontend.FilterList)-1) {
			return errors.New("can't add filter to the last position in the list")
		}
		if frontend.FilterList == nil {
			frontend.FilterList = models.Filters{}
		}
		frontend.FilterList = append(frontend.FilterList, &models.Filter{})
		copy(frontend.FilterList[id+1:], frontend.FilterList[id:])
		frontend.FilterList[id] = &rule
		return nil
	}
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.CreateFilter(id, parentType, parentName, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FilterDeleteAll(parentType, parentName string) (err error) {
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't delete filter for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.FilterList = models.Filters{}
		return nil
	}

	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
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
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get filters for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		return frontend.FilterList, nil
	}

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
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't replace filters for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.FilterList = rules
		return nil
	}
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
