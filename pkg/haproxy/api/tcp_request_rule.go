package api

import (
	"errors"
	"fmt"

	"github.com/haproxytech/client-native/v6/models"
)

func (c *clientNative) TCPRequestRuleCreate(id int64, parentType, parentName string, rule models.TCPRequestRule) error {
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't create tcp request rule for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		if id != 0 && id >= int64(len(frontend.TCPRequestRuleList)-1) {
			return errors.New("can't add tcp request rule to the last position in the list")
		}
		if frontend.TCPRequestRuleList == nil {
			frontend.TCPRequestRuleList = models.TCPRequestRules{}
		}
		frontend.TCPRequestRuleList = append(frontend.TCPRequestRuleList, &models.TCPRequestRule{})
		copy(frontend.TCPRequestRuleList[id+1:], frontend.TCPRequestRuleList[id:])
		frontend.TCPRequestRuleList[id] = &rule
		return nil
	}
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.CreateTCPRequestRule(id, parentType, parentName, &rule, c.activeTransaction, 0)
}

func (c *clientNative) TCPRequestRuleDeleteAll(parentType, parentName string) (err error) {
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't delete tcp request rule for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.TCPRequestRuleList = models.TCPRequestRules{}
		return nil
	}
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	_, rules, err := configuration.GetTCPRequestRules(parentType, parentName, c.activeTransaction)
	if err != nil {
		return err
	}
	for range rules {
		if err = configuration.DeleteTCPRequestRule(0, parentType, parentName, c.activeTransaction, 0); err != nil {
			break
		}
	}
	return err
}

func (c *clientNative) TCPRequestRulesGet(parentType, parentName string) (models.TCPRequestRules, error) {
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return nil, fmt.Errorf("can't get tcp request rules for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		return frontend.TCPRequestRuleList, nil
	}
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}

	_, rules, err := configuration.GetTCPRequestRules(parentType, parentName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (c *clientNative) TCPRequestRulesReplace(parentType, parentName string, rules models.TCPRequestRules) error {
	if parentType == "frontend" {
		frontend, exists := c.frontends[parentName]
		if !exists {
			return fmt.Errorf("can't replace tcp request rules for unexisting frontend %s : %w", parentName, ErrNotFound)
		}
		frontend.TCPRequestRuleList = rules
		return nil
	}
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}

	err = configuration.ReplaceTCPRequestRules(parentType, parentName, rules, c.activeTransaction, 0)
	if err != nil {
		return err
	}
	return nil
}
