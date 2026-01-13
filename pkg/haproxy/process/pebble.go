package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/haproxytech/client-native/v6/runtime"
	"github.com/haproxytech/client-native/v6/runtime/options"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type pebbleControl struct {
	Env               env.Env
	OSArgs            utils.OSArgs
	masterSocket      runtime.Runtime
	masterSocketValid bool
	logger            utils.Logger
}

func newPebbleControl(env env.Env, osArgs utils.OSArgs) *pebbleControl {
	pb := pebbleControl{
		Env:    env,
		OSArgs: osArgs,
		logger: utils.GetLogger(),
	}

	masterSocket, err := runtime.New(context.Background(), options.MasterSocket(MASTER_SOCKET_PATH), options.AllowDelayedStart(time.Minute, time.Second))
	if err != nil {
		pb.logger.Error(err)
		return &pb
	}
	pb.masterSocketValid = true
	pb.masterSocket = masterSocket

	return &pb
}

func (d *pebbleControl) Service(action string) (string, error) {
	if d.OSArgs.Test {
		logger.Infof("HAProxy would be %sed now", action)
		return "", nil
	}
	var cmd *exec.Cmd

	switch action {
	case "start", "stop":
		// no need to start/stop it (pebble)
		return "", nil
	case "reload":
		if d.masterSocketValid {
			msg, err := d.masterSocket.Reload()
			if err != nil {
				d.logger.Error(err)
			}
			d.logger.Debug("Reload done")
			d.logger.Debug(msg)
			return msg, err
		}
		cmd = exec.Command("pebble", "signal", "SIGUSR2", "haproxy")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return "", cmd.Run()
	default:
		return "", fmt.Errorf("unknown command '%s'", action)
	}
}

func (d *pebbleControl) UseAuxFile(useAuxFile bool) {
	// do nothing we always have it
}

func (d *pebbleControl) SetAPI(api api.HAProxyClient) {
	// unused
}
