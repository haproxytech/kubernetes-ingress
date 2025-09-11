package api

import "github.com/haproxytech/client-native/v5/models"

func (c *clientNative) CaptureCreate(frontend string, rule models.Capture) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.CreateDeclareCapture(frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) CaptureDeleteAll(frontend string) (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	_, rules, err := configuration.GetDeclareCaptures(frontend, c.activeTransaction)
	if err != nil {
		return err
	}
	for range rules {
		if err = configuration.DeleteDeclareCapture(0, frontend, c.activeTransaction, 0); err != nil {
			break
		}
	}
	return err
}

func (c *clientNative) CapturesGet(frontend string) (models.Captures, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}

	_, rules, err := configuration.GetDeclareCaptures(frontend, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return rules, nil
}
