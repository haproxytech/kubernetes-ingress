package main

import (
	"fmt"
	"sort"

	"github.com/haproxytech/models"
)

type BackendSwitchingRule struct {
	Host    string
	Path    string
	Backend string
}

func (c *HAProxyController) useBackendRuleRefresh() (err error) {
	if c.cfg.UseBackendRulesStatus == EMPTY {
		return nil
	}
	frontends := []string{"http", "https"}

	nativeAPI := c.NativeAPI

	sortedList := []string{}
	for name, _ := range c.cfg.UseBackendRules {
		sortedList = append(sortedList, name)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(sortedList))) // reverse order

	//map[string][]string
	for _, frontend := range frontends {
		err = nil
		for err == nil {
			err = nativeAPI.Configuration.DeleteBackendSwitchingRule(0, frontend, c.ActiveTransaction, 0)
		}
		for _, name := range sortedList {
			rule := c.cfg.UseBackendRules[name]
			id := int64(0)
			backendSwitchingRule := &models.BackendSwitchingRule{
				Cond:     "if",
				CondTest: fmt.Sprintf("{ req.hdr(host) -i %s } { path_beg %s }", rule.Host, rule.Path),
				Name:     rule.Backend,
				ID:       &id,
			}
			err = c.cfg.NativeAPI.Configuration.CreateBackendSwitchingRule(frontend, backendSwitchingRule, c.ActiveTransaction, 0)
			LogErr(err)
		}
	}

	return nil
}
