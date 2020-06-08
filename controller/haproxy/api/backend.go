package api

import (
	"github.com/haproxytech/models/v2"
)

func (c *clientNative) BackendsGet() (models.Backends, error) {
	_, backends, err := c.nativeAPI.Configuration.GetBackends(c.activeTransaction)
	return backends, err
}

func (c *clientNative) BackendGet(backendName string) (models.Backend, error) {
	_, backend, err := c.nativeAPI.Configuration.GetBackend(backendName, c.activeTransaction)
	if err != nil {
		return models.Backend{}, err
	}
	return *backend, nil
}

func (c *clientNative) BackendCreate(backend models.Backend) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.CreateBackend(&backend, c.activeTransaction, 0)
}

func (c *clientNative) BackendEdit(backend models.Backend) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.EditBackend(backend.Name, &backend, c.activeTransaction, 0)
}

func (c *clientNative) BackendDelete(backendName string) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.DeleteBackend(backendName, c.activeTransaction, 0)
}

func (c *clientNative) BackendHTTPRequestRuleCreate(backend string, rule models.HTTPRequestRule) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.CreateHTTPRequestRule("backend", backend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) BackendHTTPRequestRuleDeleteAll(backend string) {
	c.activeTransactionHasChanges = true
	var err error
	for err == nil {
		err = c.nativeAPI.Configuration.DeleteHTTPRequestRule(0, "backend", backend, c.activeTransaction, 0)
	}
}

func (c *clientNative) BackendHTTPResponseRuleCreate(backend string, rule models.HTTPResponseRule) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.CreateHTTPResponseRule("backend", backend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) BackendHTTPResponseRuleDeleteAll(backend string) {
	c.activeTransactionHasChanges = true
	var err error
	for err == nil {
		err = c.nativeAPI.Configuration.DeleteHTTPResponseRule(0, "backend", backend, c.activeTransaction, 0)
	}
}

func (c *clientNative) BackendServerCreate(backendName string, data models.Server) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.CreateServer(backendName, &data, c.activeTransaction, 0)
}

func (c *clientNative) BackendServerEdit(backendName string, data models.Server) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.EditServer(data.Name, backendName, &data, c.activeTransaction, 0)
}

func (c *clientNative) BackendServerDelete(backendName string, serverName string) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.DeleteServer(serverName, backendName, c.activeTransaction, 0)
}

func (c *clientNative) BackendSwitchingRuleCreate(frontend string, rule models.BackendSwitchingRule) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.CreateBackendSwitchingRule(frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) BackendSwitchingRuleDeleteAll(frontend string) {
	c.activeTransactionHasChanges = true
	var err error
	for err == nil {
		err = c.nativeAPI.Configuration.DeleteBackendSwitchingRule(0, frontend, c.activeTransaction, 0)
	}
}
