package api

import (
	"errors"
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) LogTargetCreate(id int64, parentType, parentName string, rule models.LogTarget) error {
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't create log target for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		if id != 0 && id >= int64(len(frontend.LogTargetList)-1) {
			return errors.New("can't add log target to the last position in the list")
		}
		if frontend.LogTargetList == nil {
			frontend.LogTargetList = models.LogTargets{}
		}
		frontend.LogTargetList = append(frontend.LogTargetList, &models.LogTarget{})
		copy(frontend.LogTargetList[id+1:], frontend.LogTargetList[id:])
		frontend.LogTargetList[id] = &rule
		return nil
	}

	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.CreateLogTarget(id, parentType, parentName, &rule, c.activeTransaction, 0)
}

func (c *clientNative) LogTargetDeleteAll(parentType, parentName string) (err error) {
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't delete log target for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.LogTargetList = models.LogTargets{}
		return nil
	}

	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	_, rules, err := configuration.GetLogTargets(parentType, parentName, c.activeTransaction)
	if err != nil {
		return err
	}
	for range rules {
		if err = configuration.DeleteLogTarget(0, parentType, parentName, c.activeTransaction, 0); err != nil {
			break
		}
	}
	return err
}

func (c *clientNative) LogTargetsGet(parentType, parentName string) (models.LogTargets, error) {
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get log targets for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		return frontend.LogTargetList, nil
	}
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}

	_, rules, err := configuration.GetLogTargets(parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (c *clientNative) LogTargetsReplace(parentType, parentName string, rules models.LogTargets) error {
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't replace log targets for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.LogTargetList = rules
		return nil
	}

	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}

	err = configuration.ReplaceLogTargets(parentType, parentName, rules, c.activeTransaction, 0)
	if err != nil {
		return err
	}
	return nil
}
