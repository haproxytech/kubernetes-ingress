package api

import (
	"github.com/haproxytech/client-native/v2/models"
	"github.com/haproxytech/config-parser/v4/types"
)

func (c *clientNative) BackendsGet() (models.Backends, error) {
	_, backends, err := c.nativeAPI.Configuration.GetBackends(c.activeTransaction)
	return backends, err
}

func (c *clientNative) BackendGet(backendName string) (*models.Backend, error) {
	_, backend, err := c.nativeAPI.Configuration.GetBackend(backendName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return backend, nil
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

func (c *clientNative) BackendCfgSnippetSet(backendName string, value *[]string) error {
	config, err := c.nativeAPI.Configuration.GetParser(c.activeTransaction)
	if err != nil {
		return err
	}
	if value == nil {
		err = config.Set("backend", backendName, "config-snippet", nil)
	} else {
		err = config.Set("backend", backendName, "config-snippet", types.StringSliceC{Value: *value})
	}
	if err != nil {
		c.activeTransactionHasChanges = true
	}
	return err
}

func (c *clientNative) BackendHTTPRequestRuleCreate(backend string, rule models.HTTPRequestRule) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.CreateHTTPRequestRule("backend", backend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) BackendServerDeleteAll(backendName string) bool {
	_, servers, _ := c.nativeAPI.Configuration.GetServers(backendName, c.activeTransaction)
	for _, srv := range servers {
		c.activeTransactionHasChanges = true
		_ = c.BackendServerDelete(backendName, srv.Name)
	}
	return c.activeTransactionHasChanges
}

func (c *clientNative) BackendRuleDeleteAll(backend string) {
	c.activeTransactionHasChanges = true
	var err error
	// Currently we are only using HTTPRequest rules on backend
	for err == nil {
		err = c.nativeAPI.Configuration.DeleteHTTPRequestRule(0, "backend", backend, c.activeTransaction, 0)
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

func (c *clientNative) ServerGet(serverName, backendName string) (*models.Server, error) {
	_, server, err := c.nativeAPI.Configuration.GetServer(serverName, backendName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return server, nil
}
