package main

import (
	"fmt"
	"log"

	"github.com/haproxytech/models"
)

func (c *HAProxyController) addBackendSwitchingRule(Host, Path, Backend string, frontends ...string) {
	if len(frontends) == 0 {
		frontends = []string{"http", "https"}
	}
	condTest := fmt.Sprintf("{ req.hdr(host) -i %s } { path_beg %s }", Host, Path)
	id := int64(0)
	backendSwitchingRule := &models.BackendSwitchingRule{
		Cond:     "if",
		CondTest: condTest,
		Name:     Backend,
		ID:       &id,
	}
	for _, frontend := range frontends {
		bckSwchModel, err := c.NativeAPI.Configuration.GetBackendSwitchingRules(frontend, c.ActiveTransaction)
		found := false
		if err == nil {
			data := bckSwchModel.Data
			for _, d := range data {
				if d.CondTest == condTest {
					found = true
					break
				}
			}
		}
		if !found {
			err = c.cfg.NativeAPI.Configuration.CreateBackendSwitchingRule(frontend, backendSwitchingRule, c.ActiveTransaction, 0)
			LogErr(err)
		}
	}
}

func (c *HAProxyController) removeBackendSwitchingRule(Host, Path string, frontends ...string) {
	if len(frontends) == 0 {
		frontends = []string{"http", "https"}
	}
	condTest := fmt.Sprintf("{ req.hdr(host) -i %s } { path_beg %s }", Host, Path)
	log.Println("REMOVING", condTest)
	for _, frontend := range frontends {
		bckSwchModel, err := c.NativeAPI.Configuration.GetBackendSwitchingRules(frontend, c.ActiveTransaction)
		if err == nil {
			indexShift := int64(0)
			data := bckSwchModel.Data
			for _, d := range data {
				if d.CondTest == condTest {
					err = c.NativeAPI.Configuration.DeleteBackendSwitchingRule(*d.ID-indexShift, frontend, c.ActiveTransaction, 0)
					LogErr(err)
					break
				}
			}
		}
	}
}
