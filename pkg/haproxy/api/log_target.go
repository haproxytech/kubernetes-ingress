package api

import "github.com/haproxytech/client-native/v5/models"

func (c *clientNative) LogTargetCreate(parentType, parentName string, rule models.LogTarget) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.CreateLogTarget(parentType, parentName, &rule, c.activeTransaction, 0)
}

func (c *clientNative) LogTargetDeleteAll(parentType, parentName string) (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return
	}
	c.activeTransactionHasChanges = true
	_, rules, err := configuration.GetLogTargets(parentType, parentName, c.activeTransaction)
	if err != nil {
		return
	}
	for range len(rules) {
		if err = configuration.DeleteLogTarget(0, parentType, parentName, c.activeTransaction, 0); err != nil {
			break
		}
	}
	return
}

func (c *clientNative) LogTargetsGet(parentType, parentName string) (models.LogTargets, error) {
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
