package process

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type directControl struct {
	Env        configuration.Env
	OSArgs     utils.OSArgs
	API        api.HAProxyClient
	useAuxFile bool
}

func NewDirectControl(env configuration.Env, oSArgs utils.OSArgs, api api.HAProxyClient) Process {
	return &directControl{
		Env:    env,
		OSArgs: oSArgs,
		API:    api,
	}
}

func (d *directControl) HaproxyService(action string) (err error) {
	if d.OSArgs.Test {
		logger.Infof("HAProxy would be %sed now", action)
		return nil
	}
	var cmd *exec.Cmd
	// if processErr is nil, process variable will automatically
	// hold information about a running Master HAproxy process
	process, processErr := haproxyProcess(d.Env.PIDFile)

	//nolint:gosec //checks on HAProxyBinary should be done in configuration module.
	switch action {
	case "start":
		if processErr == nil {
			logger.Error("haproxy is already running")
			return nil
		}
		cmd = exec.Command(d.Env.HAProxyBinary, "-f", d.Env.MainCFGFile)
		if d.useAuxFile {
			cmd = exec.Command(d.Env.HAProxyBinary, "-f", d.Env.MainCFGFile, "-f", d.Env.AuxCFGFile)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
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
		logger.Error(saveServerState(d.Env.StateDir, d.API))
		if processErr != nil {
			logger.Errorf("haproxy is not running, trying to start it")
			return d.HaproxyService("start")
		}
		return process.Signal(syscall.SIGUSR2)
	case "restart":
		logger.Error(saveServerState(d.Env.StateDir, d.API))
		if processErr != nil {
			logger.Errorf("haproxy is not running, trying to start it")
			return d.HaproxyService("start")
		}
		pid := strconv.Itoa(process.Pid)
		cmd = exec.Command(d.Env.HAProxyBinary, "-f", d.Env.MainCFGFile, "-sf", pid)
		if d.useAuxFile {
			cmd = exec.Command(d.Env.HAProxyBinary, "-f", d.Env.MainCFGFile, "-f", d.Env.AuxCFGFile, "-sf", pid)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	default:
		return fmt.Errorf("unknown command '%s'", action)
	}
}

func (d *directControl) UseAuxFile(useAuxFile bool) {
	d.useAuxFile = useAuxFile
}
