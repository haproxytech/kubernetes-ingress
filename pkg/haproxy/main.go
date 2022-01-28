package haproxy

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/renameio"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/config"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/process"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetLogger()

// Instance describes and controls a HAProxy Instance
type HAProxy struct {
	api.HAProxyClient
	process.Process
	config.Env
	*config.Config
}

func New(osArgs utils.OSArgs, env config.Env, cfgFile []byte, p process.Process) (h HAProxy, err error) {
	err = (&env).Init(osArgs)
	if err != nil {
		err = fmt.Errorf("failed to initialize haproxy environment: %w", err)
		return
	}
	h.Env = env

	err = renameio.WriteFile(h.MainCFGFile, cfgFile, 0755)
	if err != nil {
		err = fmt.Errorf("failed to write haproxy config file: %w", err)
		return
	}

	h.Config, err = config.New(h.Env)
	if err != nil {
		err = fmt.Errorf("failed to initialize haproxy config state: %w", err)
		return
	}

	h.HAProxyClient, err = api.New(h.CfgDir, h.MainCFGFile, h.Binary, h.RuntimeSocket)
	if err != nil {
		err = fmt.Errorf("failed to initialize haproxy API client: %w", err)
		return
	}
	if p == nil {
		h.Process = process.New(h.Env, osArgs, h.AuxCFGFile, h.HAProxyClient)
	}
	if !osArgs.Test {
		logVersion(h.Binary)
	}
	return
}

func (h *HAProxy) Refresh(cleanCrts bool) (reload bool, err error) {
	// Certs
	if cleanCrts {
		reload = h.Certificates.Refresh()
	}
	// Rules
	reload = h.SectionRules.Refresh(h.HAProxyClient) || reload
	// Maps
	reload = h.MapFiles.Refresh(h.HAProxyClient) || reload
	// Backends
	if h.SSLPassthrough {
		h.ActiveBackends[h.BackSSL] = struct{}{}
	}
	for _, rateLimitTable := range h.RateLimitTables {
		h.ActiveBackends[rateLimitTable] = struct{}{}
	}
	backends, errAPI := h.BackendsGet()
	if errAPI != nil {
		err = fmt.Errorf("unable to get configured backends")
		return
	}
	for _, backend := range backends {
		if _, ok := h.ActiveBackends[backend.Name]; !ok {
			logger.Debugf("Deleting backend '%s'", backend.Name)
			if err := h.BackendDelete(backend.Name); err != nil {
				logger.Panic(err)
			}
			annotations.RemoveBackendCfgSnippet(backend.Name)
		}
	}
	return
}

func logVersion(program string) {
	//nolint:gosec //checks of HAProxyBinary should be done in Env.Init() .
	cmd := exec.Command(program, "-v")
	res, errExec := cmd.Output()
	if errExec != nil {
		logger.Errorf("unable to get haproxy version: %s", errExec)
		return
	}
	haproxyInfo := strings.Split(string(res), "\n")
	logger.Printf("Running with %s", haproxyInfo[0])
}
