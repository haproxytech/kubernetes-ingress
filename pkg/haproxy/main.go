package haproxy

import (
	"fmt"
	"os/exec"
	"strings"

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
	if client == nil {
		h.HAProxyClient, err = api.New(h.CfgDir, h.MainCFGFile, h.Binary, h.RuntimeSocket)
		if err != nil {
			err = fmt.Errorf("failed to initialize haproxy API client: %w", err)
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
