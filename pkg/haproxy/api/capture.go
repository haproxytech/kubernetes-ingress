package api

import "github.com/haproxytech/client-native/v6/models"

func (c *clientNative) CaptureCreate(id int64, frontend string, rule models.Capture) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.CreateDeclareCapture(id, frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) CaptureDeleteAll(frontend string) (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return
	}
	_, rules, err := configuration.GetDeclareCaptures(frontend, c.activeTransaction)
	if err != nil {
		return
	}
	for range len(rules) {
		if err = configuration.DeleteDeclareCapture(0, frontend, c.activeTransaction, 0); err != nil {
			break
		}
	}
	return
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

func (c *clientNative) CapturesReplace(frontend string, rules models.Captures) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}

	err = configuration.ReplaceDeclareCaptures(frontend, rules, c.activeTransaction, 0)
	if err != nil {
		return err
	}
	return nil
}
