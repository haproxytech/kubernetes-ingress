package process

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type s6Control struct {
	Env    configuration.Env
	OSArgs utils.OSArgs
	API    api.HAProxyClient
}

func NewControlOverS6(env configuration.Env, oSArgs utils.OSArgs, api api.HAProxyClient) Process {
	return &s6Control{
		Env:    env,
		OSArgs: oSArgs,
		API:    api,
	}
}

func (d *s6Control) HaproxyService(action string) (err error) {
	if d.OSArgs.Test {
		logger.Infof("HAProxy would be %sed now", action)
		return nil
	}
	var cmd *exec.Cmd

	//nolint:gosec //checks on HAProxyBinary should be done in configuration module.
	switch action {
	case "start":
		// no need to start it is up already (s6)
		return nil
		/*if processErr == nil {
			logger.Error("haproxy is already running")
			return nil
		}
		cmd = exec.Command("s6-svc", "-u", "/var/run/s6/services/haproxy")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()*/
	case "stop":
		cmd = exec.Command("s6-svc", "-d", "/var/run/s6/services/haproxy")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	case "reload":
		logger.Error(saveServerState(d.Env.StateDir, d.API))
		cmd = exec.Command("s6-svc", "-2", "/var/run/s6/services/haproxy")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	case "restart":
		logger.Error(saveServerState(d.Env.StateDir, d.API))
		// -t terminates and s6 will start it again
		cmd = exec.Command("s6-svc", "-t", "/var/run/s6/services/haproxy")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	default:
		return fmt.Errorf("unknown command '%s'", action)
	}
}

func (d *s6Control) UseAuxFile(useAuxFile bool) {
	// do nothing we always have it
}
