package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v3/models"
	"github.com/haproxytech/config-parser/v4/types"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func (c *clientNative) BackendsGet() (models.Backends, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	_, backends, err := configuration.GetBackends(c.activeTransaction)
	return backends, err
}

func (c *clientNative) BackendGet(backendName string) (*models.Backend, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	_, backend, err := configuration.GetBackend(backendName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	c.activeBackends[backend.Name] = struct{}{}
	return backend, nil
}

func (c *clientNative) BackendCreate(backend models.Backend) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	err = configuration.CreateBackend(&backend, c.activeTransaction, 0)
	if err != nil {
		return err
	}
	c.activeBackends[backend.Name] = struct{}{}
	return nil
}

func (c *clientNative) BackendCreatePermanently(backend models.Backend) error {
	err := c.BackendCreate(backend)
	if err != nil {
		return err
	}
	c.permanentBackends[backend.Name] = struct{}{}
	return nil
}

func (c *clientNative) BackendCreateIfNotExist(backend models.Backend) (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return
	}
	c.activeTransactionHasChanges = true
	defer func() {
		if err == nil {
			c.activeBackends[backend.Name] = struct{}{}
		}
	}()

	_, _, err = configuration.GetBackend(backend.Name, c.activeTransaction)
	if err == nil {
		return
	}

	return configuration.CreateBackend(&backend, c.activeTransaction, 0)
}

func (c *clientNative) BackendEdit(backend models.Backend) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.EditBackend(backend.Name, &backend, c.activeTransaction, 0)
}

func (c *clientNative) BackendDelete(backendName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.DeleteBackend(backendName, c.activeTransaction, 0)
}

func (c *clientNative) BackendCfgSnippetSet(backendName string, value []string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	config, err := configuration.GetParser(c.activeTransaction)
	if err != nil {
		return err
	}
	if len(value) == 0 {
		err = config.Set("backend", backendName, "config-snippet", nil)
	} else {
		err = config.Set("backend", backendName, "config-snippet", types.StringSliceC{Value: value})
	}
	if err != nil {
		c.activeTransactionHasChanges = true
	}
	return err
}

func (c *clientNative) BackendHTTPRequestRuleCreate(backend string, rule models.HTTPRequestRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.CreateHTTPRequestRule("backend", backend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) BackendServerDeleteAll(backendName string) bool {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		logger := utils.GetLogger()
		logger.Error(err)
		return false
	}
	_, servers, _ := configuration.GetServers(backendName, c.activeTransaction)
	for _, srv := range servers {
		c.activeTransactionHasChanges = true
		_ = c.BackendServerDelete(backendName, srv.Name)
	}
	return c.activeTransactionHasChanges
}

func (c *clientNative) BackendRuleDeleteAll(backend string) {
	logger := utils.GetLogger()
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		logger.Error(err)
		return
	}
	c.activeTransactionHasChanges = true

	// Currently we are only using HTTPRequest rules on backend
	err = configuration.DeleteHTTPRequestRule(0, "backend", backend, c.activeTransaction, 0)
	for err != nil {
		logger.Error(err)
	}
}

func (c *clientNative) BackendServerCreate(backendName string, data models.Server) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.CreateServer(backendName, &data, c.activeTransaction, 0)
}

func (c *clientNative) BackendServerEdit(backendName string, data models.Server) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.EditServer(data.Name, backendName, &data, c.activeTransaction, 0)
}

func (c *clientNative) BackendServerDelete(backendName string, serverName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.DeleteServer(serverName, backendName, c.activeTransaction, 0)
}

func (c *clientNative) BackendSwitchingRuleCreate(frontend string, rule models.BackendSwitchingRule) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.CreateBackendSwitchingRule(frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) BackendSwitchingRuleDeleteAll(frontend string) (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return
	}
	c.activeTransactionHasChanges = true
	_, switchingRules, err := configuration.GetBackendSwitchingRules(frontend, c.activeTransaction)
	if err != nil {
		return
	}
	for i := 0; i < len(switchingRules); i++ {
		if err = configuration.DeleteBackendSwitchingRule(0, frontend, c.activeTransaction, 0); err != nil {
			break
		}
	}
	return
}

func (c *clientNative) ServerGet(serverName, backendName string) (models.Server, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return models.Server{}, err
	}
	_, server, err := configuration.GetServer(serverName, backendName, c.activeTransaction)
	if err != nil {
		return models.Server{}, err
	}
	return *server, nil
}

func (c *clientNative) BackendServersGet(backendName string) (models.Servers, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	_, servers, err := configuration.GetServers(backendName, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return servers, nil
}

func (c *clientNative) RefreshBackends() (deleted []string, err error) {
	backends, errAPI := c.BackendsGet()
	if errAPI != nil {
		err = fmt.Errorf("unable to get configured backends")
		return
	}
	for _, backend := range backends {
		if _, ok := c.permanentBackends[backend.Name]; ok {
			continue
		}
		if _, ok := c.activeBackends[backend.Name]; !ok {
			if err = c.BackendDelete(backend.Name); err != nil {
				return
			}
			deleted = append(deleted, backend.Name)
		}
	}
	c.activeBackends = map[string]struct{}{}
	return
}
