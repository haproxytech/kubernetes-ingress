package process

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type directControl struct {
	API        api.HAProxyClient
	Env        env.Env
	OSArgs     utils.OSArgs
	useAuxFile bool
}

func (d *directControl) Service(action string) (err error) {
	if d.OSArgs.Test {
		logger.Infof("HAProxy would be %sed now", action)
		return nil
	}
	var cmd *exec.Cmd
	// if processErr is nil, process variable will automatically
	// hold information about a running Master HAproxy process
	process, processErr := haproxyProcess(d.Env.PIDFile)

	masterSocketArg := d.Env.MasterSocket + ",level,admin"

	//nolint:gosec //checks on HAProxyBinary should be done in configuration module.
	switch action {
	case "start":
		if processErr == nil {
			logger.Error("haproxy is already running")
			return nil
		}
		cmd = exec.Command(d.Env.Binary, "-S", masterSocketArg, "-f", d.Env.MainCFGFile)
		if d.useAuxFile {
			cmd = exec.Command(d.Env.Binary, "-S", masterSocketArg, "-f", d.Env.MainCFGFile, "-f", d.Env.AuxCFGFile)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "stop":
		if processErr != nil {
			logger.Error("haproxy already stopped")
			return processErr
		}
		if err = process.Signal(syscall.SIGUSR1); err != nil {
			return err
		}
		_, err = process.Wait()
		return err
	case "reload":
		if processErr != nil {
			logger.Errorf("haproxy is not running, trying to start it")
			return d.Service("start")
		}
		return process.Signal(syscall.SIGUSR2)
	default:
		return fmt.Errorf("unknown command '%s'", action)
	}
}

func (d *directControl) UseAuxFile(useAuxFile bool) {
	d.useAuxFile = useAuxFile
}

func (d *directControl) SetAPI(api api.HAProxyClient) {
	d.API = api
}
