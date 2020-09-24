// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	// Configmaps
	Main        = "main"
	TCPServices = "tcpservices"
	Errorfiles  = "errorfiles"
	//frontends
	FrontendHTTP  = "http"
	FrontendHTTPS = "https"
	FrontendSSL   = "ssl"
	//Status
	ADDED    store.Status = store.ADDED
	DELETED  store.Status = store.DELETED
	ERROR    store.Status = store.ERROR
	EMPTY    store.Status = store.EMPTY
	MODIFIED store.Status = store.MODIFIED
)

var (
	HAProxyCFG        string
	HAProxyCertDir    string
	HAProxyStateDir   string
	HAProxyMapDir     string
	HAProxyErrFileDir string
	HAProxyPIDFile    string
)

var logger = utils.GetLogger()

// HAProxyController is ingress controller
type HAProxyController struct {
	k8s            *K8s
	Store          store.K8s
	PublishService *store.Service
	IngressClass   string
	cfg            Configuration
	osArgs         utils.OSArgs
	Client         api.HAProxyClient
	HAProxyCfgDir  string
	eventChan      chan SyncDataEvent
	serverlessPods map[string]int
	UpdateHandlers []UpdateHandler
}

// Return HAProxy master process if it exists.
func (c *HAProxyController) HAProxyProcess() (*os.Process, error) {
	file, err := os.Open(HAProxyPIDFile)
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

// Start initialize and run HAProxyController
func (c *HAProxyController) Start(ctx context.Context, osArgs utils.OSArgs) {

	c.osArgs = osArgs

	logger.SetLevel(osArgs.LogLevel.LogLevel)
	c.haproxyInitialize()
	c.initHandlers()

	if c.osArgs.PprofEnabled {
		logger.Error(c.Client.APIStartTransaction())
		logger.Error(c.handlePprof())
		c.refreshBackendSwitching()
		logger.Error(c.Client.APICommitTransaction())
	}

	parts := strings.Split(osArgs.PublishService, "/")
	if len(parts) == 2 {
		c.PublishService = &store.Service{
			Namespace: parts[0],
			Name:      parts[1],
			Status:    EMPTY,
			Addresses: []string{},
		}
	}

	var k8s *K8s
	var err error

	if osArgs.OutOfCluster {
		kubeconfig := filepath.Join(utils.HomeDir(), ".kube", "config")
		logger.Info("Running Controller out of K8s cluster")
		if osArgs.KubeConfig != "" {
			kubeconfig = osArgs.KubeConfig
		}
		k8s, err = GetRemoteKubernetesClient(kubeconfig)
	} else {
		k8s, err = GetKubernetesClient()
	}
	if err != nil {
		logger.Panic(err)
	}
	c.k8s = k8s

	x := k8s.API.Discovery()
	if k8sVersion, err := x.ServerVersion(); err != nil {
		logger.Panicf("Unable to get Kubernetes version: %v\n", err)
	} else {
		logger.Infof("Running on Kubernetes version: %s %s", k8sVersion.String(), k8sVersion.Platform)
	}

	c.serverlessPods = map[string]int{}
	c.eventChan = make(chan SyncDataEvent, watch.DefaultChanSize*6)
	go c.monitorChanges()
	<-ctx.Done()
}

// Sync HAProxy configuration
func (c *HAProxyController) updateHAProxy() error {
	logger.Trace("HAProxy config sync started")
	reload := false

	err := c.Client.APIStartTransaction()
	if err != nil {
		logger.Error(err)
		return err
	}
	defer func() {
		c.Client.APIDisposeTransaction()
	}()

	restart, reload := c.handleGlobalAnnotations()

	reload = c.handleDefaultService() || reload

	usedCerts := map[string]struct{}{}
	c.cfg.UsedCerts = usedCerts

	for _, namespace := range c.Store.Namespaces {
		if !namespace.Relevant {
			continue
		}
		for _, ingress := range namespace.Ingresses {
			if c.PublishService != nil && ingress.Status != DELETED {
				logger.Error(c.k8s.UpdateIngressStatus(ingress, c.PublishService))
			}
			// handle Default Backend
			if ingress.DefaultBackend != nil {
				reload = c.handlePath(namespace, ingress, &store.IngressRule{}, ingress.DefaultBackend) || reload
			}
			// handle Ingress rules
			for _, rule := range ingress.Rules {
				for _, path := range rule.Paths {
					reload = c.handlePath(namespace, ingress, rule, path) || reload
				}
			}
			//handle certs
			ingressSecrets := map[string]struct{}{}
			for _, tls := range ingress.TLS {
				if _, ok := ingressSecrets[tls.SecretName.Value]; !ok {
					ingressSecrets[tls.SecretName.Value] = struct{}{}
					reload = c.handleTLSSecret(*ingress, *tls, usedCerts) || reload
				}
			}

			logger.Error(c.handleRateLimiting(ingress))
			logger.Error(c.handleRequestCapture(ingress))
			logger.Error(c.handleRequestSetHdr(ingress))
			logger.Error(c.handleResponseSetHdr(ingress))
			logger.Error(c.handleBlacklisting(ingress))
			logger.Error(c.handleWhitelisting(ingress))
			logger.Error(c.handleHTTPRedirect(ingress))
		}
	}

	var r bool
	for _, handler := range c.UpdateHandlers {
		r, err = handler.Update(c.Store, c.cfg, c.Client)
		logger.Error(err)
		reload = reload || r
	}

	r, err = c.cfg.MapFiles.Refresh()
	logger.Error(err)
	reload = reload || r

	r, err = c.handleTCPServices()
	logger.Error(err)
	reload = reload || r

	reload = c.FrontendHTTPReqsRefresh() || reload

	reload = c.FrontendHTTPRspsRefresh() || reload

	reload = c.FrontendTCPreqsRefresh() || reload

	reload = c.BackendHTTPReqsRefresh() || reload

	reload = c.refreshBackendSwitching() || reload

	err = c.Client.APICommitTransaction()
	if err != nil {
		logger.Error(err)
		return err
	}
	c.clean()
	if restart {
		if err := c.haproxyService("restart"); err != nil {
			logger.Error(err)
		} else {
			logger.Info("HAProxy restarted")
		}
		return nil
	}
	if reload {
		if err := c.haproxyService("reload"); err != nil {
			logger.Error(err)
		} else {
			logger.Info("HAProxy reloaded")
		}
	}

	logger.Trace("HAProxy config sync terminated")
	return nil
}

//HAProxyInitialize runs HAProxy for the first time so native client can have access to it
func (c *HAProxyController) haproxyInitialize() {
	if HAProxyCFG == "" {
		HAProxyCFG = filepath.Join(c.HAProxyCfgDir, "haproxy.cfg")
	}
	if HAProxyPIDFile == "" {
		HAProxyPIDFile = "/var/run/haproxy.pid"
	}
	if _, err := os.Stat(HAProxyCFG); err != nil {
		logger.Panic(err)
	}
	if HAProxyCertDir == "" {
		HAProxyCertDir = filepath.Join(c.HAProxyCfgDir, "certs")
	}
	if HAProxyMapDir == "" {
		HAProxyMapDir = filepath.Join(c.HAProxyCfgDir, "maps")
	}
	if HAProxyErrFileDir == "" {
		HAProxyErrFileDir = filepath.Join(c.HAProxyCfgDir, "errors")
	}
	if HAProxyStateDir == "" {
		HAProxyStateDir = "/var/state/haproxy/"
	}
	for _, d := range []string{HAProxyCertDir, HAProxyMapDir, HAProxyErrFileDir, HAProxyStateDir} {
		err := os.MkdirAll(d, 0755)
		if err != nil {
			logger.Panic(err)
		}
	}
	_, err := os.Create(filepath.Join(HAProxyStateDir, "global"))
	logger.Err(err)

	cmd := exec.Command("sh", "-c", "haproxy -v")
	haproxyInfo, err := cmd.Output()
	if err == nil {
		haproxyInfo := strings.Split(string(haproxyInfo), "\n")
		logger.Printf("Running with %s", haproxyInfo[0])
	} else {
		logger.Error(err)
	}

	logger.Infof("Starting HAProxy with %s", HAProxyCFG)
	logger.Panic(c.haproxyService("start"))

	hostname, err := os.Hostname()
	logger.Error(err)
	logger.Infof("Running on %s", hostname)

	c.Client, err = api.Init(HAProxyCFG, "haproxy", "/var/run/haproxy-runtime-api.sock")
	if err != nil {
		logger.Panic(err)
	}

	c.cfg.Init(HAProxyMapDir)
}

// Handle HAProxy daemon via Master process
func (c *HAProxyController) haproxyService(action string) (err error) {
	if c.osArgs.Test {
		logger.Infof("HAProxy would be %sed now", action)
		return nil
	}

	var cmd *exec.Cmd
	// if processErr is nil, process variable will automatically
	// hold information about a running Master HAproxy process
	process, processErr := c.HAProxyProcess()

	switch action {
	case "start":
		if processErr == nil {
			logger.Error(fmt.Errorf("haproxy is already running"))
			return nil
		}
		cmd = exec.Command("haproxy", "-W", "-f", HAProxyCFG, "-p", HAProxyPIDFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	case "stop":
		if processErr != nil {
			logger.Error(fmt.Errorf("haproxy  already stopped"))
			return processErr
		}
		return process.Signal(syscall.SIGUSR1)
	case "reload":
		logger.Error(c.saveServerState())
		if processErr != nil {
			logger.Errorf("haproxy is not running, trying to start it")
			return c.haproxyService("start")
		}
		return process.Signal(syscall.SIGUSR2)
	case "restart":
		logger.Error(c.saveServerState())
		if processErr != nil {
			logger.Errorf("haproxy is not running, trying to start it")
			return c.haproxyService("start")
		}
		pid := strconv.Itoa(process.Pid)
		cmd = exec.Command("haproxy", "-W", "-f", HAProxyCFG, "-p", HAProxyPIDFile, "-sf", pid)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	default:
		return fmt.Errorf("unknown command '%s'", action)
	}
}

// Saves HAProxy servers state so it is retrieved after reload.
func (c *HAProxyController) saveServerState() error {
	result, err := c.Client.ExecuteRaw("show servers state")
	if err != nil {
		return err
	}
	var f *os.File
	if f, err = os.Create(HAProxyStateDir + "global"); err != nil {
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

func (c *HAProxyController) clean() {
	c.Store.Clean()
	c.cfg.Clean()
	if c.PublishService != nil {
		c.PublishService.Status = EMPTY
	}
}
