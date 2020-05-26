package controller

import (
	"github.com/haproxytech/models/v2"
)

func (c *HAProxyController) apiStartTransaction() error {
	version, errVersion := c.NativeAPI.Configuration.GetVersion("")
	if errVersion != nil || version < 1 {
		//silently fallback to 1
		version = 1
	}
	transaction, err := c.NativeAPI.Configuration.StartTransaction(version)
	c.ActiveTransaction = transaction.ID
	c.ActiveTransactionHasChanges = false
	return err
}

func (c *HAProxyController) apiCommitTransaction() error {
	if !c.ActiveTransactionHasChanges {
		if err := c.NativeAPI.Configuration.DeleteTransaction(c.ActiveTransaction); err != nil {
			return err
		}
		return nil
	}
	_, err := c.NativeAPI.Configuration.CommitTransaction(c.ActiveTransaction)
	return err
}

func (c *HAProxyController) apiDisposeTransaction() {
	c.ActiveTransaction = ""
	c.ActiveTransactionHasChanges = false
}

func (c *HAProxyController) backendsGet() (models.Backends, error) {
	_, backends, err := c.NativeAPI.Configuration.GetBackends(c.ActiveTransaction)
	return backends, err
}

func (c *HAProxyController) backendGet(backendName string) (models.Backend, error) {
	_, backend, err := c.NativeAPI.Configuration.GetBackend(backendName, c.ActiveTransaction)
	if err != nil {
		return models.Backend{}, err
	}
	return *backend, nil
}

func (c *HAProxyController) backendCreate(backend models.Backend) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.CreateBackend(&backend, c.ActiveTransaction, 0)
}

func (c *HAProxyController) backendEdit(backend models.Backend) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.EditBackend(backend.Name, &backend, c.ActiveTransaction, 0)
}

func (c *HAProxyController) backendDelete(backendName string) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.DeleteBackend(backendName, c.ActiveTransaction, 0)
}

func (c *HAProxyController) backendHTTPRequestRuleCreate(backend string, rule models.HTTPRequestRule) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.CreateHTTPRequestRule("backend", backend, &rule, c.ActiveTransaction, 0)
}

func (c *HAProxyController) backendHTTPRequestRuleDeleteAll(backend string) {
	c.ActiveTransactionHasChanges = true
	var err error
	for err == nil {
		err = c.NativeAPI.Configuration.DeleteHTTPRequestRule(0, "backend", backend, c.ActiveTransaction, 0)
	}
}

func (c *HAProxyController) backendServerCreate(backendName string, data models.Server) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.CreateServer(backendName, &data, c.ActiveTransaction, 0)
}

func (c *HAProxyController) backendServerEdit(backendName string, data models.Server) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.EditServer(data.Name, backendName, &data, c.ActiveTransaction, 0)
}

func (c *HAProxyController) backendServerDelete(backendName string, serverName string) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.DeleteServer(serverName, backendName, c.ActiveTransaction, 0)
}

func (c *HAProxyController) backendSwitchingRuleCreate(frontend string, rule models.BackendSwitchingRule) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.CreateBackendSwitchingRule(frontend, &rule, c.ActiveTransaction, 0)
}

func (c *HAProxyController) backendSwitchingRuleDeleteAll(frontend string) {
	c.ActiveTransactionHasChanges = true
	var err error
	for err == nil {
		err = c.NativeAPI.Configuration.DeleteBackendSwitchingRule(0, frontend, c.ActiveTransaction, 0)
	}
}

func (c *HAProxyController) frontendCreate(frontend models.Frontend) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.CreateFrontend(&frontend, c.ActiveTransaction, 0)
}

func (c *HAProxyController) frontendDelete(frontendName string) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.DeleteFrontend(frontendName, c.ActiveTransaction, 0)
}

func (c *HAProxyController) frontendsGet() (models.Frontends, error) {
	_, frontends, err := c.NativeAPI.Configuration.GetFrontends(c.ActiveTransaction)
	return frontends, err
}

func (c *HAProxyController) frontendGet(frontendName string) (models.Frontend, error) {
	_, frontend, err := c.NativeAPI.Configuration.GetFrontend(frontendName, c.ActiveTransaction)
	if err != nil {
		return models.Frontend{}, err
	}
	return *frontend, err
}

func (c *HAProxyController) frontendEdit(frontend models.Frontend) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.EditFrontend(frontend.Name, &frontend, c.ActiveTransaction, 0)
}

func (c *HAProxyController) frontendBindsGet(frontend string) (models.Binds, error) {
	_, binds, err := c.NativeAPI.Configuration.GetBinds(frontend, c.ActiveTransaction)
	return binds, err
}

func (c *HAProxyController) frontendBindCreate(frontend string, bind models.Bind) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.CreateBind(frontend, &bind, c.ActiveTransaction, 0)
}

func (c *HAProxyController) frontendBindEdit(frontend string, bind models.Bind) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.EditBind(bind.Name, frontend, &bind, c.ActiveTransaction, 0)
}

func (c *HAProxyController) frontendBindDeleteAll(frontend string) error {
	c.ActiveTransactionHasChanges = true
	binds, _ := c.frontendBindsGet(frontend)
	for _, bind := range binds {
		err := c.NativeAPI.Configuration.DeleteBind(bind.Name, frontend, c.ActiveTransaction, 0)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *HAProxyController) frontendHTTPRequestRuleDeleteAll(frontend string) {
	c.ActiveTransactionHasChanges = true
	var err error
	for err == nil {
		err = c.NativeAPI.Configuration.DeleteHTTPRequestRule(0, "frontend", frontend, c.ActiveTransaction, 0)
	}
}

func (c *HAProxyController) frontendHTTPResponseRuleDeleteAll(frontend string) {
	c.ActiveTransactionHasChanges = true
	var err error
	for err == nil {
		err = c.NativeAPI.Configuration.DeleteHTTPResponseRule(0, "frontend", frontend, c.ActiveTransaction, 0)
	}
}

func (c *HAProxyController) frontendHTTPRequestRuleCreate(frontend string, rule models.HTTPRequestRule) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.CreateHTTPRequestRule("frontend", frontend, &rule, c.ActiveTransaction, 0)
}

func (c *HAProxyController) frontendHTTPResponseRuleCreate(frontend string, rule models.HTTPResponseRule) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.CreateHTTPResponseRule("frontend", frontend, &rule, c.ActiveTransaction, 0)
}

func (c *HAProxyController) frontendTCPRequestRuleDeleteAll(frontend string) {
	c.ActiveTransactionHasChanges = true
	var err error
	for err == nil {
		err = c.NativeAPI.Configuration.DeleteTCPRequestRule(0, "frontend", frontend, c.ActiveTransaction, 0)
	}
}

func (c *HAProxyController) frontendTCPRequestRuleCreate(frontend string, rule models.TCPRequestRule) error {
	c.ActiveTransactionHasChanges = true
	return c.NativeAPI.Configuration.CreateTCPRequestRule("frontend", frontend, &rule, c.ActiveTransaction, 0)
}
