package process

import (
	"bufio"
	"os"
	"strconv"
	"syscall"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetLogger()

// MUST be the same as in fs/etc/s6-overlay/s6-rc.d/haproxy/run
const MASTER_SOCKET_PATH = "/var/run/haproxy-master.sock" //nolint:stylecheck

type Process interface {
	Service(action string) (msg string, err error)
	UseAuxFile(useAuxFile bool)
	SetAPI(api api.HAProxyClient)
}

func New(env env.Env, osArgs utils.OSArgs, auxCfgFile string, api api.HAProxyClient) (p Process) { //nolint:ireturn
	switch {
	case osArgs.UseWiths6Overlay:
		p = newS6Control(api, env, osArgs)
	case osArgs.UseWithPebble:
		p = newPebbleControl(env, osArgs)
	default:
		p = &directControl{
			Env:    env,
			OSArgs: osArgs,
			API:    api,
		}
		if _, err := os.Stat(auxCfgFile); err == nil {
			p.UseAuxFile(true)
		}
		_, _ = p.Service("start")
	}
	return p
}

// Return HAProxy master process if it exists.
func haproxyProcess(pidFile string) (*os.Process, error) {
	file, err := os.Open(pidFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Scan()
	pid, err := strconv.Atoi(scanner.Text())
	if err != nil {
		return nil, err
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}
	err = process.Signal(syscall.Signal(0))
	return process, err
}
