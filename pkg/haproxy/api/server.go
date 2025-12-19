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

func (c *clientNative) BackendServersDeleteAllInMaint() error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	_, backends, err := configuration.GetBackends(c.activeTransaction)
	if err != nil {
		return err
	}
	var errs utils.Errors
	for _, backend := range backends {
		_, servers, err := configuration.GetServers("backend", backend.Name, c.activeTransaction)
		if err != nil {
			return err
		}
		for _, server := range servers {
			if server.Maintenance == "enabled" {
				logger.Debugf("[CONFIG] [BACKEND] [SERVER] [DEL] server %s/%s: deleting server in configuration file", backend.Name, server.Name)
				// Delete from configuration
				err = configuration.DeleteServer(server.Name, "backend", backend.Name, c.activeTransaction, 0)
				if err != nil {
					errs.Add(err)
				}
				// Delete from interal staorage
				logger.Tracef("[CONFIG] [BACKEND] [SERVER] [DEL] server %s/%s: deleting server in storage", backend.Name, server.Name)

				err = c.BackendServerDelete(backend.Name, server.Name)
				if err != nil {
					errs.Add(err)
				}
			}
		}
	}
	return errs.Result()
}
