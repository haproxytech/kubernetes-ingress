package api

import (
	"errors"
	"strconv"
	"strings"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/client-native/v6/runtime"

	"github.com/haproxytech/kubernetes-ingress/pkg/controller/constants"
	"github.com/haproxytech/kubernetes-ingress/pkg/metrics"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var ErrMapNotFound = errors.New("map not found")

type RuntimeServerData struct {
	BackendName string
	ServerName  string
	IP          string
	State       string
	Port        int
}

func (c *clientNative) ExecuteRaw(command string) (result string, err error) {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return "", err
	}
	result, err = runtime.ExecuteRaw(command)
	return result, err
}

func (c *clientNative) SetServerAddrAndState(servers []RuntimeServerData) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		return nil
	}
	backendNameSize := len(servers[0].BackendName)
	oneServerCommandSize := 75 + 2*backendNameSize
	size := oneServerCommandSize * len(servers)
	if size > BufferSize {
		size = BufferSize
	}

	var sb strings.Builder
	sb.Grow(size)
	var cmdBuilder strings.Builder
	cmdBuilder.Grow(oneServerCommandSize)
	for _, server := range servers {
		// if new commands are added recalculate oneServerCommandSize
		cmdBuilder.WriteString("set server ")
		cmdBuilder.WriteString(server.BackendName)
		cmdBuilder.WriteString("/")
		cmdBuilder.WriteString(server.ServerName)
		cmdBuilder.WriteString(" addr ")
		cmdBuilder.WriteString(server.IP)
		if server.Port > 0 {
			cmdBuilder.WriteString(" port ")
			cmdBuilder.WriteString(strconv.Itoa(server.Port))
		}
		cmdBuilder.WriteString(";set server ")
		cmdBuilder.WriteString(server.BackendName)
		cmdBuilder.WriteString("/")
		cmdBuilder.WriteString(server.ServerName)
		cmdBuilder.WriteString(" state ")
		cmdBuilder.WriteString(server.State)
		cmdBuilder.WriteString(";")
		// if new commands are added recalculate oneServerCommandSize

		if sb.Len()+cmdBuilder.Len() >= BufferSize {
			err = c.runRaw(runtime, sb, server.BackendName)
			if err != nil {
				return err
			}
			sb.Reset()
			sb.Grow(size)
		}
		sb.WriteString(cmdBuilder.String())
		cmdBuilder.Reset()
		cmdBuilder.Grow(oneServerCommandSize)
	}
	if sb.Len() > 0 {
		err = c.runRaw(runtime, sb, servers[0].BackendName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *clientNative) runRaw(runtime runtime.Runtime, sb strings.Builder, backendName string) error {
	logger := utils.GetLogger()
	pmm := metrics.New()
	result, err := runtime.ExecuteRaw(sb.String())
	if err != nil {
		pmm.UpdateRuntimeMetrics(metrics.ObjectServer, err)
		return err
	}
	if len(result) > 5 {
		switch result[0:4] {
		case "[3]:", "[2]:", "[1]:", "[0]:":
			logger.Errorf("[RUNTIME] [BACKEND] [SOCKET] backend %s', server slots adjustment ?", backendName)
			logger.Tracef("[RUNTIME] [BACKEND] [SOCKET] backend %s: Error: '%s', server slots adjustment ?", backendName, result)
			err := errors.New("runtime update failed for " + backendName)
			pmm.UpdateRuntimeMetrics(metrics.ObjectServer, err)
			return err
		}
	}
	pmm.UpdateRuntimeMetrics(metrics.ObjectServer, nil)
	return nil
}

// SyncBackendSrvs syncs states and addresses of a backend servers with corresponding endpoints.
func (c *clientNative) SyncBackendSrvs(backend *store.RuntimeBackend) error {
	logger := utils.GetLogger()
	if backend.Name == "" {
		return nil
	}
	logger.Tracef("[RUNTIME] [BACKEND] [SERVER] updating backend  %s for haproxy servers update (address and state) through socket", backend.Name)
	haproxySrvs := backend.HAProxySrvs
	endpoints := backend.Endpoints
	logger.Tracef("[RUNTIME] [BACKEND] [SERVER] backend %s: list of servers %+v", backend.Name, haproxySrvs)
	logger.Tracef("[RUNTIME] [BACKEND] [SERVER] backend %s: list of endpoints %+v", backend.Name, endpoints)
	// Disable stale entries from HAProxySrvs
	for i, srv := range haproxySrvs {
		srvEndpoint := store.RuntimeEndpoint{Address: srv.Address, Port: srv.Port}
		if _, ok := endpoints[srvEndpoint]; ok {
			delete(endpoints, srvEndpoint)
		} else {
			haproxySrvs[i].Address = ""
			haproxySrvs[i].Port = 1
			haproxySrvs[i].Modified = true
		}
	}

	logger.Tracef("[RUNTIME] [BACKEND] [SERVER] backend %s: list of servers after treatment  %+v", backend.Name, haproxySrvs)
	logger.Tracef("[RUNTIME] [BACKEND] [SERVER] backend %s: list of endpoints after treatment  %+v", backend.Name, endpoints)

	// New addresses: reuse a MAINT server or add one at runtime.
	errNew := c.SyncNewServers(backend, endpoints)
	if errNew != nil {
		backend.DynUpdateFailed = true
		return errNew
	}

	// Dynamically updates HAProxy backend servers  with HAProxySrvs content
	// Updates the addresses and put them in MAINT or READY
	runtimeServerData := make([]RuntimeServerData, 0, len(haproxySrvs))
	for _, srv := range haproxySrvs {
		if !srv.Modified || srv.Deleted {
			continue
		}
		if srv.Address == "" {
			logger.Debugf("[RUNTIME] [BACKEND] [SERVER] [SOCKET] backend %s: server '%s' changed status to %v", backend.Name, srv.Name, "maint")
			runtimeServerData = append(runtimeServerData, RuntimeServerData{
				BackendName: backend.Name,
				ServerName:  srv.Name,
				IP:          "127.0.0.1",
				Port:        1,
				State:       "maint",
			})
		} else {
			logger.Debugf("[RUNTIME] [BACKEND] [SERVER] [SOCKET] backend %s: server '%s': addr '%s' changed status to %v", backend.Name, srv.Name, srv.Address, "ready")
			runtimeServerData = append(runtimeServerData, RuntimeServerData{
				BackendName: backend.Name,
				ServerName:  srv.Name,
				IP:          srv.Address,
				Port:        int(srv.Port),
				State:       "ready",
			})
		}
	}
	err := c.SetServerAddrAndState(runtimeServerData)
	if err != nil {
		backend.DynUpdateFailed = true
		return err
	}

	// Last, try to delete the MAINT servers
	err = c.DeleteMaintServers(backend)
	if err != nil {
		return err
	}

	return nil
}

// DeleteMaintServers will delete the servers in MAINT in backend.HAProxySrvs through the runtime
// It might fail, it's normal
//
//	Delete a removable server attached to the backend <backend>. A removable
//
// server is the server which satisfies all of these conditions :
// - not referenced by other configuration elements
// - must already be in maintenance (see "disable server")
// - must not have any active or idle connections
// If any of these conditions is not met, the command will fail.
// The runtime commands status are stored in the RuntimeUpdateTracker
func (c *clientNative) DeleteMaintServers(backend *store.RuntimeBackend) error {
	for _, server := range backend.HAProxySrvs {
		if server.Address == "" {
			err := c.BackendServerRuntimeDelete(backend.Name, server.Name)
			if err == nil {
				server.Deleted = true
			}
		}
	}
	return nil
}

// reusableMaintServers lists MAINT servers still present in the running process.
func reusableMaintServers(srvs map[string]*store.HAProxySrv) map[string]*store.HAProxySrv {
	reusable := make(map[string]*store.HAProxySrv)
	for name, srv := range srvs {
		if srv.Address == "" && !srv.Deleted {
			reusable[name] = srv
		}
	}
	return reusable
}

// SyncNewServers takes care of new addresses (= new servers) on a backend
// - if a server with the same IP/PORT is found in MAINT, re-use it
// - if not add a server through runtime
// newAddresses contains the addresses that are added
// backend contains the current state of the runtime backend
// backend.HAProxySrvs is updated with the new/updated servers
// - new if no maint one was found
// - updated from maint to ready if a maint was found
// The runtime commands status are stored in the RuntimeUpdateTracker and a reload will be issued if the command fails (for add/update, not for delete as deletion failure is normal)
func (c *clientNative) SyncNewServers(backend *store.RuntimeBackend, endpoints store.RuntimeEndpoints) error {
	haproxySrvs := backend.HAProxySrvs
	if haproxySrvs == nil {
		haproxySrvs = make(map[string]*store.HAProxySrv)
	}

	disabledSrvNames := reusableMaintServers(backend.HAProxySrvs)

	cnBackend, err := c.BackendGet(backend.Name)
	if err != nil {
		// Backend already removed from the config: nothing to add servers to.
		utils.GetLogger().Debugf("backend %s: %v, skipping runtime server creation", backend.Name, err)
		return nil
	}
	config, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	_, sectionDefaults, err := config.GetDefaultsSection(constants.DefaultsSectionName, c.activeTransaction)
	if err != nil {
		return err
	}
	var defaultServer *models.DefaultServer
	if sectionDefaults != nil {
		defaultServer = sectionDefaults.DefaultServer
	}

	for newRuntimeEndpoint := range endpoints {
		// First check if a server already exists in MAINT, then re-use it and set it to READY
		newSrvName := newRuntimeEndpoint.ComputeServerName()

		if disabledSrv, ok := disabledSrvNames[newSrvName]; ok {
			// We found a MAINT server, re-use it and set it to READY
			utils.GetLogger().Debugf("backend %s newAddress %s - Re-using server %s and set it to ready", backend.Name, newRuntimeEndpoint.Address, newSrvName)
			delete(disabledSrvNames, newSrvName)
			// Set those servers to ready
			disabledSrv.Modified = true
			disabledSrv.Address = newRuntimeEndpoint.Address
			disabledSrv.Port = int64(newRuntimeEndpoint.Port)
			continue
		}

		// no server in MAINT found, we create a new one
		srv := models.Server{
			Name:         newSrvName,
			Port:         utils.PtrInt64(newRuntimeEndpoint.Port),
			Address:      newRuntimeEndpoint.Address,
			ServerParams: models.ServerParams{Maintenance: "disabled"},
		}

		utils.GetLogger().Debugf("backend %s newAddress %s - Creating server %s ", backend.Name, newRuntimeEndpoint.Address, newSrvName)
		// RUNTIME SOCKET call
		errAPI := c.BackendServerRuntimeCreate(cnBackend, srv, defaultServer)
		if errAPI == nil {
			// If and error occurs, the runtime command status Failure is stored in RuntimeUpdateTracker and a reload will be issued
			haproxySrv := &store.HAProxySrv{
				Name:     srv.Name,
				Address:  newRuntimeEndpoint.Address,
				Modified: true,
				Port:     newRuntimeEndpoint.Port,
			}
			haproxySrvs[srv.Name] = haproxySrv
		}
	}

	backend.HAProxySrvs = haproxySrvs
	return nil
}

func (c *clientNative) CertEntryCreate(filename string) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.NewCertEntry(filename)
}

func (c *clientNative) CertEntrySet(filename string, payload []byte) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.SetCertEntry(filename, string(payload))
}

func (c *clientNative) CertEntryCommit(filename string) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.CommitCertEntry(filename)
}

func (c *clientNative) CertEntryAbort(filename string) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.AbortCertEntry(filename)
}

func (c *clientNative) CrtListEntryAdd(crtList string, entry runtime.CrtListEntry) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.AddCrtListEntry(crtList, entry)
}

func (c *clientNative) CrtListEntryDelete(crtList, filename string, linenumber *int64) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.DeleteCrtListEntry(crtList, filename, nil)
}

func (c *clientNative) CertEntryDelete(filename string) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.DeleteCertEntry(filename)
}

func (c *clientNative) CertAuthEntryCreate(filename string) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.NewCAFile(filename)
}

func (c *clientNative) CertAuthEntrySet(filename string, payload []byte) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.SetCAFile(filename, string(payload))
}

func (c *clientNative) CertAuthEntryCommit(filename string) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.CommitCAFile(filename)
}

func (c *clientNative) CertAuthEntryAbort(filename string) error {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	return runtime.AbortCAFile(filename)
}
