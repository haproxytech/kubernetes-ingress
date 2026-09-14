package api

import (
	"fmt"
	"slices"
	"strings"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

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
		return fmt.Errorf("server %s does not exist in backend %s, %w", data.Name, backendName, ErrNotFound)
	}
	_, exists = backend.Servers[data.Name]
	if !exists {
		return fmt.Errorf("server %s does not exist in backend %s, %w", data.Name, backendName, ErrNotFound)
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

// maintServers lists, per backend, the in-memory servers in maintenance.
func (c *clientNative) maintServers() map[string][]string {
	result := map[string][]string{}
	for backendName, backend := range c.backends {
		names := make([]string, 0, len(backend.Servers))
		for serverName, server := range backend.Servers {
			if server.Maintenance == "enabled" {
				names = append(names, serverName)
			}
		}
		if len(names) > 0 {
			slices.Sort(names)
			result[backendName] = names
		}
	}
	return result
}

// BackendServersDeleteAllInMaint drops MAINT servers from the file and memory.
// A failed runtime deletion is covered later: the server comes back as disabled and triggers a reload.
func (c *clientNative) BackendServersDeleteAllInMaint() error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	var errs utils.Errors
	for backendName, serverNames := range c.maintServers() {
		for _, serverName := range serverNames {
			logger.Debugf("[CONFIG] [BACKEND] [SERVER] [DEL] server %s/%s: deleting server in configuration file", backendName, serverName)
			// Already absent from the file is the desired end state.
			if err := configuration.DeleteServer(serverName, "backend", backendName, c.activeTransaction, 0); err != nil &&
				!strings.Contains(err.Error(), "does not exist") {
				errs.Add(err)
			}
			errs.Add(c.BackendServerDelete(backendName, serverName))
		}
	}
	return errs.Result()
}
