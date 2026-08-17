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

type gopherdControl struct {
	API               api.HAProxyClient
	Env               env.Env
	OSArgs            utils.OSArgs
	masterSocket      runtime.Runtime
	masterSocketValid bool
	logger            utils.Logger
}

func newGopherdControl(api api.HAProxyClient, env env.Env, osArgs utils.OSArgs) *gopherdControl {
	gc := gopherdControl{
		API:    api,
		Env:    env,
		OSArgs: osArgs,
		logger: utils.GetLogger(),
	}

	masterSocket, err := runtime.New(context.Background(), options.MasterSocket(MASTER_SOCKET_PATH), options.AllowDelayedStart(time.Minute, time.Second))
	if err != nil {
		gc.logger.Error(err)
		return &gc
	}
	gc.masterSocketValid = true
	gc.masterSocket = masterSocket

	return &gc
}

func (d *gopherdControl) Service(action string) (string, error) {
	if d.OSArgs.Test {
		logger.Infof("HAProxy would be %sed now", action)
		return "", nil
	}
	var cmd *exec.Cmd

	switch action {
	case "start", "stop":
		// gopherd owns start/stop
		return "", nil
	case "reload":
		if d.masterSocketValid {
			msg, err := d.masterSocket.Reload()
			if err == nil {
				d.logger.Debug(msg)
				return "", nil
			}
			d.logger.Error(err)
			return msg, err
		}

		cmd = exec.Command("gopherd", "signal", "haproxy", "SIGUSR2")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return "", cmd.Run()
	default:
		return "", fmt.Errorf("unknown command '%s'", action)
	}
}

func (d *gopherdControl) UseAuxFile(useAuxFile bool) {
	// do nothing we always have it
}

func (d *gopherdControl) SetAPI(api api.HAProxyClient) {
	d.API = api
}
