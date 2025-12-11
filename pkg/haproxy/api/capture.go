package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

// func (c *clientNative) CaptureCreate(id int64, frontend string, rule models.Capture) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	return configuration.CreateDeclareCapture(id, frontend, &rule, c.activeTransaction, 0)
// }

func (c *clientNative) CaptureCreate(id int64, frontendName string, rule models.Capture) error {
	frontend := c.frontends[frontendName]
	if frontend == nil {
		return fmt.Errorf("can't create capture for unexisting frontend %s : %w", frontendName, ErrNotFound)
	}
	frontend.CaptureList = append(frontend.CaptureList, &rule)
	return nil
}

// func (c *clientNative) CaptureDeleteAll(frontend string) (err error) {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	_, rules, err := configuration.GetDeclareCaptures(frontend, c.activeTransaction)
// 	if err != nil {
// 		return err
// 	}
// 	for range rules {
// 		if err = configuration.DeleteDeclareCapture(0, frontend, c.activeTransaction, 0); err != nil {
// 			break
// 		}
// 	}
// 	return err
// }

func (c *clientNative) CaptureDeleteAll(frontendName string) (err error) {
	frontend := c.frontends[frontendName]
	if frontend == nil {
		return fmt.Errorf("can't delete capture for unexisting frontend %s : %w", frontendName, ErrNotFound)
	}
	frontend.CaptureList = nil
	return nil
}

// func (c *clientNative) CapturesGet(frontend string) (models.Captures, error) {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return nil, err
// 	}

// 	_, rules, err := configuration.GetDeclareCaptures(frontend, c.activeTransaction)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return rules, nil
// }

func (c *clientNative) CapturesGet(frontendName string) (models.Captures, error) {
	frontend := c.frontends[frontendName]
	if frontend == nil {
		return nil, fmt.Errorf("can't get captures for unexisting frontend %s : %w", frontendName, ErrNotFound)
	}
	return frontend.CaptureList, nil
}

// func (c *clientNative) CapturesReplace(frontend string, rules models.Captures) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}

// 	err = configuration.ReplaceDeclareCaptures(frontend, rules, c.activeTransaction, 0)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

func (c *clientNative) CapturesReplace(frontendName string, rules models.Captures) error {
	frontend := c.frontends[frontendName]
	if frontend == nil {
		return fmt.Errorf("can't replace captures for unexisting frontend %s : %w", frontendName, ErrNotFound)
	}
	frontend.CaptureList = rules
	return nil
}
