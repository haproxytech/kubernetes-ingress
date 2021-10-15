package api

import (
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v3/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var ErrMapNotFound = fmt.Errorf("map not found")

func (c *clientNative) ExecuteRaw(command string) (result []string, err error) {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.ExecuteRaw(command)
}

func (c *clientNative) SetServerAddr(backendName string, serverName string, ip string, port int) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.SetServerAddr(backendName, serverName, ip, port)
}

func (c *clientNative) SetServerState(backendName string, serverName string, state string) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.SetServerState(backendName, serverName, state)
}

func (c *clientNative) SetMapContent(mapFile string, payload []string) error {
	var mapVer, mapPath string
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	mapVer, err = runtime.PrepareMap(mapFile)
	if err != nil {
		if strings.HasPrefix(err.Error(), "maps dir doesn't exists") {
			err = ErrMapNotFound
		}
		err = fmt.Errorf("error preparing map file: %w", err)
		return err
	}
	mapPath, err = runtime.GetMapsPath(mapFile)
	if err != nil {
		err = fmt.Errorf("error getting map path: %w", err)
		return err
	}
	for i := 0; i < len(payload); i++ {
		_, err = runtime.ExecuteRaw(fmt.Sprintf("add map @%s %s <<\n%s\n", mapVer, mapPath, payload[i]))
		if err != nil {
			err = fmt.Errorf("error loading map payload: %w", err)
			return err
		}
	}
	err = runtime.CommitMap(mapVer, mapFile)
	if err != nil {
		err = fmt.Errorf("error committing map file: %w", err)
	}
	return err
}

func (c *clientNative) GetMap(mapFile string) (*models.Map, error) {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.GetMap(mapFile)
}

// SyncBackendSrvs syncs states and addresses of a backend servers with corresponding endpoints.
func (c *clientNative) SyncBackendSrvs(backend *store.RuntimeBackend, portUpdated bool) error {
	if backend.Name == "" {
		return nil
	}
	haproxySrvs := backend.HAProxySrvs
	addresses := backend.Endpoints.Addresses
	// Disable stale entries from HAProxySrvs
	// and provide list of Disabled Srvs
	var disabled []*store.HAProxySrv
	var errors utils.Errors
	for i, srv := range haproxySrvs {
		srv.Modified = srv.Modified || portUpdated
		if _, ok := addresses[srv.Address]; ok {
			delete(addresses, srv.Address)
		} else {
			haproxySrvs[i].Address = ""
			haproxySrvs[i].Modified = true
			disabled = append(disabled, srv)
		}
	}

	// Configure new Addresses in available HAProxySrvs
	for newAddr := range addresses {
		if len(disabled) == 0 {
			break
		}
		disabled[0].Address = newAddr
		disabled[0].Modified = true
		disabled = disabled[1:]
		delete(addresses, newAddr)
	}
	// Dynamically updates HAProxy backend servers  with HAProxySrvs content
	var addrErr, stateErr error
	for _, srv := range haproxySrvs {
		if !srv.Modified {
			continue
		}
		if srv.Address == "" {
			// logger.Tracef("server '%s/%s' changed status to %v", newEndpoints.BackendName, srv.Name, "maint")
			addrErr = c.SetServerAddr(backend.Name, srv.Name, "127.0.0.1", 0)
			stateErr = c.SetServerState(backend.Name, srv.Name, "maint")
		} else {
			// logger.Tracef("server '%s/%s' changed status to %v", newEndpoints.BackendName, srv.Name, "ready")
			addrErr = c.SetServerAddr(backend.Name, srv.Name, srv.Address, int(backend.Endpoints.Port))
			stateErr = c.SetServerState(backend.Name, srv.Name, "ready")
		}
		if addrErr != nil || stateErr != nil {
			backend.DynUpdateFailed = true
			errors.Add(addrErr)
			errors.Add(stateErr)
		}
	}
	return errors.Result()
}
