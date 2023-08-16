package process

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type pebbleControl struct {
	Env    env.Env
	OSArgs utils.OSArgs
}

func (d *pebbleControl) Service(action string) error {
	if d.OSArgs.Test {
		logger.Infof("HAProxy would be %sed now", action)
		return nil
	}
	var cmd *exec.Cmd

	switch action {
	case "start":
		// no need to start it is up already (pebble)
		return nil
	case "stop":
		// no need to stop it (pebble)
		return nil
	case "reload":
		cmd = exec.Command("pebble", "signal", "SIGUSR2", "haproxy")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "restart":
		cmd = exec.Command("pebble", "restart", "haproxy")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("unknown command '%s'", action)
	}
}

func (d *pebbleControl) UseAuxFile(useAuxFile bool) {
	// do nothing we always have it
}

func (d *pebbleControl) SetAPI(api api.HAProxyClient) {
	// unused
}
