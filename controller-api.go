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

func (c HAProxyController) backendCreate(backend models.Backend) error {
	return c.NativeAPI.Configuration.CreateBackend(&backend, c.ActiveTransaction, 0)
}

func (c HAProxyController) backendEdit(backendName string, backend models.Backend) error {
	return c.NativeAPI.Configuration.EditBackend(backendName, &backend, c.ActiveTransaction, 0)
}

func (c HAProxyController) backendDelete(backendName string) error {
	return c.NativeAPI.Configuration.DeleteBackend(backendName, c.ActiveTransaction, 0)
}

func (c HAProxyController) backendServerCreate(backendName string, data models.Server) error {
	return c.NativeAPI.Configuration.CreateServer(backendName, &data, c.ActiveTransaction, 0)
}

func (c HAProxyController) backendServerEdit(backendName string, data models.Server) error {
	return c.NativeAPI.Configuration.EditServer(data.Name, backendName, &data, c.ActiveTransaction, 0)
}

func (c HAProxyController) backendServerDelete(backendName string, serverName string) error {
	return c.NativeAPI.Configuration.DeleteServer(serverName, backendName, c.ActiveTransaction, 0)
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

func (c HAProxyController) frontendACLAdd(frontend string, acl models.ACL) error {
	return c.NativeAPI.Configuration.CreateACL("frontend", frontend, &acl, c.ActiveTransaction, 0)
}

func (c HAProxyController) frontendACLDelete(frontend string, index int64) error {
	return c.NativeAPI.Configuration.DeleteACL(index, "frontend", frontend, c.ActiveTransaction, 0)
}

func (c HAProxyController) frontendACLsGet(frontend string) (models.Acls, error) {
	_, acls, err := c.NativeAPI.Configuration.GetACLs("frontend", frontend, c.ActiveTransaction)
	return acls, err
}

func (c HAProxyController) frontendHTTPRequestRuleDeleteAll(frontend string) {
	var err error
	for err == nil {
		err = c.NativeAPI.Configuration.DeleteHTTPRequestRule(0, "frontend", frontend, c.ActiveTransaction, 0)
	}
}

func (c HAProxyController) frontendHTTPRequestRuleCreate(frontend string, rule models.HTTPRequestRule) error {
	return c.NativeAPI.Configuration.CreateHTTPRequestRule("frontend", frontend, &rule, c.ActiveTransaction, 0)
}

func (c HAProxyController) frontendTCPRequestRuleDeleteAll(frontend string) {
	var err error
	for err == nil {
		err = c.NativeAPI.Configuration.DeleteTCPRequestRule(0, "frontend", frontend, c.ActiveTransaction, 0)
	}
}

func (c HAProxyController) frontendTCPRequestRuleCreate(frontend string, rule models.TCPRequestRule) error {
	return c.NativeAPI.Configuration.CreateTCPRequestRule("frontend", frontend, &rule, c.ActiveTransaction, 0)
}
