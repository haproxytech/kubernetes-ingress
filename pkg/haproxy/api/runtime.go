package api

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/client-native/v6/runtime"

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

func (c *clientNative) SetMapContent(mapFile string, payload []string) error {
	var mapVer, mapPath string
	pmm := metrics.New()
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
	for i := range payload {
		_, err = runtime.ExecuteRaw(fmt.Sprintf("add map @%s %s <<\n%s\n", mapVer, mapPath, payload[i]))
		pmm.UpdateRuntimeMetrics(metrics.ObjectMap, err)
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
	logger := utils.GetLogger()
	if backend.Name == "" {
		return nil
	}
	logger.Tracef("[RUNTIME] [BACKEND] [SERVER] updating backend  %s for haproxy servers update (address and state) through socket", backend.Name)
	haproxySrvs := backend.HAProxySrvs
	addresses := backend.Endpoints.Addresses
	logger.Tracef("[RUNTIME] [BACKEND] [SERVER] backend %s: list of servers %+v", backend.Name, haproxySrvs)
	logger.Tracef("[RUNTIME] [BACKEND] [SERVER] backend %s: list of endpoints addresses %+v", backend.Name, addresses)
	// Disable stale entries from HAProxySrvs
	// and provide list of Disabled Srvs
	var disabled []*store.HAProxySrv
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

	logger.Tracef("[RUNTIME] [BACKEND] [SERVER] backend %s: list of servers after treatment  %+v", backend.Name, haproxySrvs)
	logger.Tracef("[RUNTIME] [BACKEND] [SERVER] backend %s: list of endpoints addresses after treatment  %+v", backend.Name, addresses)

	// Dynamically updates HAProxy backend servers  with HAProxySrvs content
	runtimeServerData := make([]RuntimeServerData, 0, len(haproxySrvs))
	for _, srv := range haproxySrvs {
		if !srv.Modified {
			continue
		}
		if srv.Address == "" {
			logger.Tracef("[RUNTIME] [BACKEND] [SERVER] [SOCKET] backend %s: server '%s' changed status to %v", backend.Name, srv.Name, "maint")
			runtimeServerData = append(runtimeServerData, RuntimeServerData{
				BackendName: backend.Name,
				ServerName:  srv.Name,
				IP:          "127.0.0.1",
				Port:        0,
				State:       "maint",
			})
		} else {
			logger.Tracef("[RUNTIME] [BACKEND] [SERVER] [SOCKET] backend %s: server '%s': addr '%s' changed status to %v", backend.Name, srv.Name, srv.Address, "ready")
			runtimeServerData = append(runtimeServerData, RuntimeServerData{
				BackendName: backend.Name,
				ServerName:  srv.Name,
				IP:          srv.Address,
				Port:        int(backend.Endpoints.Port),
				State:       "ready",
			})
		}
	}
	err := c.SetServerAddrAndState(runtimeServerData)
	if err != nil {
		backend.DynUpdateFailed = true
		return err
	}
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
