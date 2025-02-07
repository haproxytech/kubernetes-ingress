package api

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/haproxytech/client-native/v5/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var ErrNotFound = errors.New("not found")

func (c *clientNative) BackendsGet() models.Backends {
	backends := models.Backends(make([]*models.Backend, len(c.backends)))
	i := 0
	for _, backend := range c.backends {
		backends[i] = &backend.BackendBase
		i++
	}
	return backends
}

func (c *clientNative) BackendGet(backendName string) (*models.Backend, error) {
	oldBackend, ok := c.backends[backendName]
	if ok {
		return &oldBackend.BackendBase, nil
	}
	return nil, fmt.Errorf("backend %s not found", backendName)
}

func (c *clientNative) BackendCreatePermanently(backend models.Backend) {
	c.BackendCreateOrUpdate(backend)
	newBackend := c.backends[backend.Name]
	newBackend.Permanent = true
	c.backends[backend.Name] = newBackend
}

func (c *clientNative) BackendCreateIfNotExist(backend models.Backend) {
	existingBackend := c.backends[backend.Name]
	existingBackend.Used = true
	c.backends[backend.Name] = existingBackend
	if c.BackendUsed(backend.Name) {
		return
	}
	c.BackendCreateOrUpdate(backend)
}

func (c *clientNative) BackendCreateOrUpdate(backend models.Backend) (diff map[string][]interface{}, created bool) {
	oldBackend, ok := c.backends[backend.Name]
	if !ok {
		c.backends[backend.Name] = Backend{
			BackendBase: backend,
			Used:        true,
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

// To remove ?
func (c *clientNative) BackendHTTPRequestRuleCreate(backendName string, rule models.HTTPRequestRule) error {
	backend, exists := c.backends[backendName]
	if !exists {
		return fmt.Errorf("can't add http request rule for unexisting backend %s, %w", backendName, ErrNotFound)
	}
	backend.HTTPRequestsRules = append(backend.HTTPRequestsRules, &rule)
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
	err = configuration.DeleteHTTPRequestRule(0, "backend", backend, c.activeTransaction, 0)
	for err != nil {
		logger.Error(err)
	}
}

func (c *clientNative) BackendServerCreateOrUpdate(backendName string, data models.Server) error {
	backend, exists := c.backends[backendName]
	if !exists {
		return fmt.Errorf("can't create server for unexisting backend %s", backendName)
	}
	if data.Name == "" {
		return fmt.Errorf("can't create unnamed server in backend %s", backendName)
	}

	if backend.Servers == nil {
		backend.Servers = map[string]models.Server{}
	}
	backend.Servers[data.Name] = data
	c.backends[backendName] = backend
	return nil
}

func (c *clientNative) BackendServerCreate(backendName string, data models.Server) error {
	backend, exists := c.backends[backendName]
	if !exists {
		return fmt.Errorf("can't create server for unexisting backend %s", backendName)
	}
	if data.Name == "" {
		return fmt.Errorf("can't create unnamed server in backend %s", backendName)
	}

	_, exists = backend.Servers[data.Name]
	if exists {
		return fmt.Errorf("can't create already existing server %s in backend %s", data.Name, backendName)
	}
	if backend.Servers == nil {
		backend.Servers = map[string]models.Server{}
	}
	backend.Servers[data.Name] = data
	c.backends[backendName] = backend
	return nil
}

func (c *clientNative) BackendServerEdit(backendName string, data models.Server) error {
	backend, exists := c.backends[backendName]
	if !exists {
		return fmt.Errorf("can't edit server for unexisting backend %s, %w", backendName, ErrNotFound)
	}
	if data.Name == "" {
		return fmt.Errorf("can't edit unnamed server in backend %s", backendName)
	}

	if backend.Servers == nil {
		return fmt.Errorf("can't edit unexisting server %s in backend %s, %w", data.Name, backendName, ErrNotFound)
	}
	_, exists = backend.Servers[data.Name]
	if !exists {
		return fmt.Errorf("can't edit unexisting server %s in backend %s, %w", data.Name, backendName, ErrNotFound)
	}
	backend.Servers[data.Name] = data
	c.backends[backendName] = backend
	return nil
}

func (c *clientNative) BackendServerDelete(backendName string, serverName string) error {
	backend, exists := c.backends[backendName]
	if !exists {
		return fmt.Errorf("can't edit server for unexisting backend %s", backendName)
	}
	if serverName == "" {
		return fmt.Errorf("can't edit unnamed server in backend %s", backendName)
	}

	_, exists = backend.Servers[serverName]
	if !exists {
		return fmt.Errorf("can't delete unexisting server %s in backend %s", serverName, backendName)
	}
	delete(backend.Servers, serverName)
	c.backends[backendName] = backend
	return nil
}

func (c *clientNative) BackendServerGet(serverName, backendName string) (*models.Server, error) {
	backend, exists := c.backends[backendName]
	if !exists {
		return nil, fmt.Errorf("can't get server %s for unexisting backend %s", serverName, backendName)
	}
	if serverName == "" {
		return nil, fmt.Errorf("can't get unnamed server in backend %s", backendName)
	}

	server, exists := backend.Servers[serverName]
	if !exists {
		return nil, nil //nolint:golint,nilnil
	}
	return &server, nil
}

func (c *clientNative) BackendServersGet(backendName string) (models.Servers, error) {
	backend, exists := c.backends[backendName]
	if !exists {
		return nil, fmt.Errorf("can't get server for unexisting backend %s", backendName)
	}
	servers := models.Servers(make([]*models.Server, len(backend.Servers)))
	i := 0
	for _, server := range backend.Servers {
		servers[i] = &server
		i++
	}
	slices.SortFunc(servers, func(a, b *models.Server) int {
		lenDiff := len(a.Name) - len(b.Name)
		if lenDiff != 0 {
			return lenDiff
		}
		return strings.Compare(a.Name, b.Name)
	})
	return servers, nil
}

// This function tests if a backend is existing
// Check if you're not rather looking for BackendUsed function.
func (c *clientNative) BackendExists(backendName string) (exists bool) {
	_, exists = c.backends[backendName]
	return
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
