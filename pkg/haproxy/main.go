package haproxy

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/renameio"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/process"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/route"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetLogger()

var SSLPassthrough bool

// HAProxy holds haproxy config state
type HAProxy struct {
	api.HAProxyClient
	process.Process
	maps.Maps
	rules.Rules
	certs.Certificates
	env.Env
}

func New(osArgs utils.OSArgs, env env.Env, cfgFile []byte, p process.Process, client api.HAProxyClient, rules rules.Rules) (h HAProxy, err error) {
	err = (&env).Init(osArgs)
	if err != nil {
		err = fmt.Errorf("failed to initialize haproxy environment: %w", err)
		return
	}
	h.Env = env

	if osArgs.External {
		cfgFile = []byte(strings.ReplaceAll(string(cfgFile), "/var/run/haproxy-runtime-api.sock", h.RuntimeSocket))
		cfgFile = []byte(strings.ReplaceAll(string(cfgFile), "pidfile /var/run/haproxy.pid", "pidfile "+h.PIDFile))
	}

	err = renameio.WriteFile(h.MainCFGFile, cfgFile, 0o755)
	if err != nil {
		err = fmt.Errorf("failed to write haproxy config file: %w", err)
		return
	}
	persistentMaps := []maps.Name{
		route.SNI,
		route.HOST,
		route.PATH_EXACT,
		route.PATH_PREFIX,
	}
	if h.Maps, err = maps.New(env.MapsDir, persistentMaps); err != nil {
		err = fmt.Errorf("failed to initialize haproxy maps: %w", err)
		return
	}
	if p == nil {
		h.Process = process.New(h.Env, osArgs, h.AuxCFGFile, h.HAProxyClient)
	}
	if client != nil {
		h.HAProxyClient = client
	} else {
		if err = h.Service("start"); err != nil {
			err = fmt.Errorf("failed to start haproxy service: %w", err)
			return
		}
		if err = h.ConnectToAPI(); err != nil {
			err = fmt.Errorf("failed to connect to haproxy API: %w", err)
			return
		}
	}
	h.Process.SetAPI(h.HAProxyClient)
	if h.Certificates, err = certs.New(env.Certs); err != nil {
		err = fmt.Errorf("failed to initialize haproxy certificates: %w", err)
		return
	}
	h.Rules = rules
	if !osArgs.Test {
		logVersion(h.Binary)
	}
	return
}

func (h HAProxy) ConnectToAPI() (err error) {
	if h.HAProxyClient == nil {
		timer := time.NewTimer(h.Env.HaproxyStartupTime)
		_, errStat := os.Stat(h.RuntimeSocket)
		if errStat != nil {
			// wait for runtime socket to be available
			watcher, errWatcher := fsnotify.NewWatcher()
			if errWatcher != nil {
				err = fmt.Errorf("unable to initialize fnotify to detect runtime socket creation: %w", errWatcher)
				return
			}
			defer watcher.Close()
			errRuntimeSocket := make(chan error)
			go func() {
				for {
					select {
					case event, ok := <-watcher.Events:
						if !ok {
							return
						}
						if event.Name == h.RuntimeSocket && event.Op == fsnotify.Create {
							logger.Printf("Runtime socket successfully detected")
							errRuntimeSocket <- nil
						}
					case err, ok := <-watcher.Errors:
						if !ok {
							return
						}
						errRuntimeSocket <- fmt.Errorf("unable to detect runtime socket startup: %w", err)
					case <-timer.C:
						errRuntimeSocket <- fmt.Errorf("runtime socket not found after %g seconds", h.Env.HaproxyStartupTime.Seconds())
					}
				}
			}()
			err = watcher.Add(filepath.Dir(h.RuntimeSocket))
			if err != nil {
				err = fmt.Errorf("unable to create watcher for runtime socket dir: %w", err)
				return
			}
			err = <-errRuntimeSocket
			if err != nil {
				return
			}
		}
		// check for successful socket connection
		for {
			conn, errRuntimeConnect := net.Dial("unix", h.RuntimeSocket)
			if errRuntimeConnect == nil {
				logger.Printf("Runtime socket connection successfully established")
				conn.Close()
				break
			}
			select {
			case <-timer.C:
				err = fmt.Errorf("unable to connect to runtime socket after %g seconds", h.Env.HaproxyStartupTime.Seconds())
				return
			case <-time.After(time.Duration(500) * time.Millisecond):
				continue
			}
		}
		// connect to runtime socket
		h.HAProxyClient, err = api.New(h.CfgDir, h.MainCFGFile, h.Binary, h.RuntimeSocket)
		if err != nil {
			err = fmt.Errorf("failed to initialize haproxy API client: %w", err)
			return
		}
	}
	return nil
}

func (h HAProxy) Clean() {
	SSLPassthrough = false
	h.CleanMaps()
	h.CleanCerts()
	h.CleanRules()
}

func logVersion(program string) {
	// checks of HAProxyBinary should be done in Env.Init() .
	cmd := exec.Command(program, "-v")
	res, errExec := cmd.Output()
	if errExec != nil {
		logger.Errorf("unable to get haproxy version: %s", errExec)
		return
	}
	haproxyInfo := strings.Split(string(res), "\n")
	logger.Printf("Running with %s", haproxyInfo[0])
}
