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

type Process interface {
	Service(action string) (err error)
	UseAuxFile(useAuxFile bool)
}

func New(env env.Env, osArgs utils.OSArgs, auxCfgFile string, api api.HAProxyClient) (p Process) {
	if osArgs.UseWiths6Overlay {
		p = &s6Control{
			Env:    env,
			OSArgs: osArgs,
			API:    api,
		}
	} else {
		p = &directControl{
			Env:    env,
			OSArgs: osArgs,
			API:    api,
		}
		if _, err := os.Stat(auxCfgFile); err == nil {
			p.UseAuxFile(true)
		}
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

// Saves HAProxy servers state so it is retrieved after reload.
func saveServerState(stateDir string, api api.HAProxyClient) error {
	result, err := api.ExecuteRaw("show servers state")
	if err != nil {
		return err
	}
	var f *os.File
	if f, err = os.Create(stateDir + "global"); err != nil {
		logger.Error(err)
		return err
	}
	defer f.Close()
	if _, err = f.Write([]byte(result[0])); err != nil {
		logger.Error(err)
		return err
	}
	if err = f.Sync(); err != nil {
		logger.Error(err)
		return err
	}
	if err = f.Close(); err != nil {
		logger.Error(err)
		return err
	}
	return nil
}
