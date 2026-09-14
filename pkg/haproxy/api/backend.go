package api

import (
	"errors"
	"fmt"

	parser "github.com/haproxytech/client-native/v6/config-parser"
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var ErrNotFound = errors.New("not found")

func (c *clientNative) BackendsGet() models.Backends {
	backends := models.Backends(make([]*models.Backend, len(c.backends)))
	i := 0
	for _, backend := range c.backends {
		backends[i] = &backend.Backend
		i++
	}
	return backends
}

func (c *clientNative) BackendGet(backendName string) (*models.Backend, error) {
	oldBackend, ok := c.backends[backendName]
	if ok {
		return &oldBackend.Backend, nil
	}
	return nil, fmt.Errorf("backend %s not found", backendName)
}

func (c *clientNative) BackendCreatePermanently(backend models.BackendBase) {
	c.BackendCreateOrUpdate(backend)
	newBackend := c.backends[backend.Name]
	newBackend.Permanent = true
	c.backends[backend.Name] = newBackend
}

func (c *clientNative) BackendCreateIfNotExist(backend models.BackendBase) {
	existingBackend := c.backends[backend.Name]
	existingBackend.Used = true
	c.backends[backend.Name] = existingBackend
	if c.BackendUsed(backend.Name) {
		return
	}
	c.BackendCreateOrUpdate(backend)
}

func (c *clientNative) BackendCreateOrUpdate(backend models.BackendBase) (diff map[string][]interface{}, created bool) {
	oldBackend, ok := c.backends[backend.Name]
	if !ok {
		c.backends[backend.Name] = Backend{
			Backend: models.Backend{BackendBase: backend},
			Used:    true,
		}
		return nil, true
	}

	diff = oldBackend.BackendBase.Diff(backend)
	oldBackend.BackendBase = backend
	oldBackend.Used = true
	c.backends[backend.Name] = oldBackend
	return diff, false
}

func (c *clientNative) BackendDelete(backendName string) {
	backend, exists := c.backends[backendName]
	if !exists {
		return
	}
	backend.Used = false
	backend.Permanent = false
	c.backends[backendName] = backend
}

func (c *clientNative) BackendCfgSnippetSet(backendName string, value []string) error {
	backend, exists := c.backends[backendName]
	if !exists {
		return fmt.Errorf("backend %s : %w", backendName, ErrNotFound)
	}

	backend.ConfigSnippets = value
	c.backends[backendName] = backend
	return nil
}

func (c *clientNative) BackendServerDeleteAll(backendName string) error {
	backend, exists := c.backends[backendName]
	if !exists {
		return fmt.Errorf("can't delete servers from unexisting backend %s", backendName)
	}
	backend.Servers = nil
	c.backends[backendName] = backend
	return nil
}

func (c *clientNative) BackendRuleDeleteAll(backend string) {
	logger := utils.GetLogger()
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		logger.Error(err)
		return
	}

	// Currently we are only using HTTPRequest rules on backend
	err = configuration.DeleteHTTPRequestRule(0, string(parser.Backends), backend, c.activeTransaction, 0)
	for err != nil {
		logger.Error(err)
	}
}

// This function tests if a backend is existing
// Check if you're not rather looking for BackendUsed function.
func (c *clientNative) BackendExists(backendName string) (exists bool) {
	_, exists = c.backends[backendName]
	return exists
}

func (c *clientNative) BackendDeleteAllUnnecessary() ([]string, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}

	var errs utils.Errors
	var backendDeleted []string //nolint:prealloc
	for _, backend := range c.backends {
		// if a backend is not permanent and has not been "viewed" in the transacton then remove it.
		if backend.Used || backend.Permanent {
			continue
		}
		backendName := backend.BackendBase.Name
		delete(c.backends, backendName)
		_ = configuration.DeleteBackend(backendName, c.activeTransaction, 0)
		backendDeleted = append(backendDeleted, backendName)
	}
	return backendDeleted, errs.Result()
}

// This function tests if a backend is existing AND IT'S USED.
func (c *clientNative) BackendUsed(backendName string) bool {
	backend, exists := c.backends[backendName]
	if !exists {
		return false
	}
	return backend.Used
}
