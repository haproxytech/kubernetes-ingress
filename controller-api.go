package main

import (
	"github.com/haproxytech/models"
)

func (c HAProxyController) backendGet(backendName string) (models.Backend, error) {
	_, backend, err := c.NativeAPI.Configuration.GetBackend(backendName, c.ActiveTransaction)
	if err != nil {
		return models.Backend{}, err
	}
	return *backend, nil
}

func (c HAProxyController) backendEdit(backendName string, backend models.Backend) error {
	return c.NativeAPI.Configuration.EditBackend(backendName, &backend, c.ActiveTransaction, 0)
}

func (c HAProxyController) backendDelete(backendName string) error {
	return c.NativeAPI.Configuration.DeleteBackend(backendName, c.ActiveTransaction, 0)
}

func (c HAProxyController) backendSwitchingRuleCreate(frontend string, rule models.BackendSwitchingRule) error {
	return c.cfg.NativeAPI.Configuration.CreateBackendSwitchingRule(frontend, &rule, c.ActiveTransaction, 0)
}

func (c HAProxyController) backendSwitchingRuleDeleteAll(frontend string) {
	var err error
	for err == nil {
		err = c.NativeAPI.Configuration.DeleteBackendSwitchingRule(0, frontend, c.ActiveTransaction, 0)
	}
}
