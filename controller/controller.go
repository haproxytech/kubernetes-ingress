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

	clientnative "github.com/haproxytech/client-native/v2"
	"github.com/haproxytech/client-native/v2/configuration"
	"github.com/haproxytech/client-native/v2/runtime"
	parser "github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"k8s.io/apimachinery/pkg/watch"
)

// HAProxyController is ingress controller
type HAProxyController struct {
	k8s                         *K8s
	cfg                         Configuration
	osArgs                      utils.OSArgs
	NativeAPI                   *clientnative.HAProxyClient
	ActiveTransaction           string
	ActiveTransactionHasChanges bool
	HAProxyCfgDir               string
	eventChan                   chan SyncDataEvent
	serverlessPods              map[string]int
	Logger                      utils.Logger
}

// Return Parser of current configuration (for config-parser usage)
func (c *HAProxyController) ActiveConfiguration() (*parser.Parser, error) {
	if c.ActiveTransaction == "" {
		return nil, fmt.Errorf("no active transaction")
	}
	return c.NativeAPI.Configuration.GetParser(c.ActiveTransaction)
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

	c.haproxyInitialize()

	var k8s *K8s
	var err error

	if osArgs.OutOfCluster {
		kubeconfig := filepath.Join(utils.HomeDir(), ".kube", "config")
		if osArgs.KubeConfig != "" {
			kubeconfig = osArgs.KubeConfig
		}
		k8s, err = GetRemoteKubernetesClient(kubeconfig)
	} else {
		k8s, err = GetKubernetesClient()
	}
	if err != nil {
		c.Logger.Panic(err)
	}
	c.k8s = k8s

	x := k8s.API.Discovery()
	if k8sVersion, err := x.ServerVersion(); err != nil {
		c.Logger.Panicf("Unable to get Kubernetes version: %v\n", err)
	} else {
		c.Logger.Infof("Running on Kubernetes version: %s %s", k8sVersion.String(), k8sVersion.Platform)
	}

	c.serverlessPods = map[string]int{}
	c.eventChan = make(chan SyncDataEvent, watch.DefaultChanSize*6)
	go c.monitorChanges()
	<-ctx.Done()
}

// Sync HAProxy configuration
func (c *HAProxyController) updateHAProxy() error {
	reload := false

	err := c.apiStartTransaction()
	if err != nil {
		c.Logger.Error(err)
		return err
	}
	defer func() {
		c.apiDisposeTransaction()
	}()

	restart, reload := c.handleGlobalAnnotations()

	r, err := c.handleDefaultService()
	c.Logger.Error(err)
	reload = reload || r

	usedCerts := map[string]struct{}{}

	for _, namespace := range c.cfg.Namespace {
		if !namespace.Relevant {
			continue
		}
		for _, ingress := range namespace.Ingresses {
			if c.cfg.PublishService != nil && ingress.Status != DELETED {
				c.Logger.Error(c.k8s.UpdateIngressStatus(ingress, c.cfg.PublishService))
			}
			// handle Default Backend
			if ingress.DefaultBackend != nil {
				r, err = c.handlePath(namespace, ingress, &IngressRule{}, ingress.DefaultBackend)
				c.Logger.Error(err)
				reload = reload || r
			}
			// handle Ingress rules
			for _, rule := range ingress.Rules {
				for _, path := range rule.Paths {
					r, err = c.handlePath(namespace, ingress, rule, path)
					reload = reload || r
					c.Logger.Error(err)
				}
			}
			//handle certs
			ingressSecrets := map[string]struct{}{}
			for _, tls := range ingress.TLS {
				if _, ok := ingressSecrets[tls.SecretName.Value]; !ok {
					ingressSecrets[tls.SecretName.Value] = struct{}{}
					r = c.handleTLSSecret(*ingress, *tls, usedCerts)
					reload = reload || r
				}
			}

			c.Logger.Error(c.handleRateLimiting(ingress))
			c.Logger.Error(c.handleRequestCapture(ingress))
			c.Logger.Error(c.handleRequestSetHdr(ingress))
			c.Logger.Error(c.handleResponseSetHdr(ingress))
			c.Logger.Error(c.handleBlacklisting(ingress))
			c.Logger.Error(c.handleWhitelisting(ingress))
			c.Logger.Error(c.handleHTTPRedirect(ingress))
		}
	}

	c.Logger.Error(c.handleProxyProtocol())

	r = c.handleDefaultCertificate(usedCerts)
	reload = reload || r

	r = c.handleHTTPS(usedCerts)
	reload = reload || r

	reload = c.FrontendHTTPReqsRefresh() || reload

	reload = c.FrontendHTTPRspsRefresh() || reload

	reload = c.FrontendTCPreqsRefresh() || reload

	reload = c.BackendHTTPReqsRefresh() || reload

	r, err = c.cfg.MapFiles.Refresh()
	c.Logger.Error(err)
	reload = reload || r

	r, err = c.handleTCPServices()
	c.Logger.Error(err)
	reload = reload || r

	r = c.refreshBackendSwitching()
	reload = reload || r

	err = c.apiCommitTransaction()
	if err != nil {
		c.Logger.Error(err)
		return err
	}
	c.cfg.Clean()
	if restart {
		if err := c.haproxyService("restart"); err != nil {
			c.Logger.Error(err)
		} else {
			c.Logger.Info("HAProxy restarted")
		}
		return nil
	}
	if reload {
		if err := c.haproxyService("reload"); err != nil {
			c.Logger.Error(err)
		} else {
			c.Logger.Info("HAProxy reloaded")
		}
	}
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
		c.Logger.Panic(err)
	}
	if HAProxyCertDir == "" {
		HAProxyCertDir = filepath.Join(c.HAProxyCfgDir, "certs")
	}
	if HAProxyMapDir == "" {
		HAProxyMapDir = filepath.Join(c.HAProxyCfgDir, "maps")
	}
	if HAProxyStateDir == "" {
		HAProxyStateDir = "/var/state/haproxy/"
	}
	for _, d := range []string{HAProxyCertDir, HAProxyMapDir, HAProxyStateDir} {
		err := os.MkdirAll(d, 0755)
		if err != nil {
			c.Logger.Panic(err)
		}
	}
	_, err := os.Create(filepath.Join(HAProxyStateDir, "global"))
	c.Logger.Err(err)

	cmd := exec.Command("sh", "-c", "haproxy -v")
	haproxyInfo, err := cmd.Output()
	if err == nil {
		haproxyInfo := strings.Split(string(haproxyInfo), "\n")
		c.Logger.Printf("Running with %s", haproxyInfo[0])
	} else {
		c.Logger.Error(err)
	}

	c.Logger.Infof("Starting HAProxy with %s", HAProxyCFG)
	c.Logger.Panic(c.haproxyService("start"))

	hostname, err := os.Hostname()
	c.Logger.Error(err)
	c.Logger.Infof("Running on %s", hostname)

	runtimeClient := runtime.Client{}
	err = runtimeClient.InitWithSockets(map[int]string{
		0: "/var/run/haproxy-runtime-api.sock",
	})
	if err != nil {
		c.Logger.Panic(err)
	}

	confClient := configuration.Client{}
	err = confClient.Init(configuration.ClientParams{
		ConfigurationFile:      HAProxyCFG,
		PersistentTransactions: false,
		Haproxy:                "haproxy",
	})
	if err != nil {
		c.Logger.Panic(err)
	}

	c.NativeAPI = &clientnative.HAProxyClient{
		Configuration: &confClient,
		Runtime:       &runtimeClient,
	}

	c.cfg.Init(c.osArgs, HAProxyMapDir)

}

// Handle HAProxy daemon via Master process
func (c *HAProxyController) haproxyService(action string) (err error) {
	if c.osArgs.Test {
		c.Logger.Infof("HAProxy would be %sed now", action)
		return nil
	}

	var cmd *exec.Cmd
	// if processErr is nil, process variable will automatically
	// hold information about a running Master HAproxy process
	process, processErr := c.HAProxyProcess()

	switch action {
	case "start":
		if processErr == nil {
			c.Logger.Error(fmt.Errorf("haproxy is already running"))
			return nil
		}
		cmd = exec.Command("haproxy", "-W", "-f", HAProxyCFG, "-p", HAProxyPIDFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	case "stop":
		if processErr != nil {
			c.Logger.Error(fmt.Errorf("haproxy  already stopped"))
			return processErr
		}
		return process.Signal(syscall.SIGUSR1)
	case "reload":
		c.Logger.Error(c.saveServerState())
		if processErr != nil {
			c.Logger.Errorf("haproxy is not running, trying to start it")
			return c.haproxyService("start")
		}
		return process.Signal(syscall.SIGUSR2)
	case "restart":
		c.Logger.Error(c.saveServerState())
		if processErr != nil {
			c.Logger.Errorf("haproxy is not running, trying to start it")
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
	result, err := c.NativeAPI.Runtime.ExecuteRaw("show servers state")
	if err != nil {
		return err
	}
	var f *os.File
	if f, err = os.Create(HAProxyStateDir + "global"); err != nil {
		c.Logger.Error(err)
		return err
	}
	defer f.Close()
	if _, err = f.Write([]byte(result[0])); err != nil {
		c.Logger.Error(err)
		return err
	}
	if err = f.Sync(); err != nil {
		c.Logger.Error(err)
		return err
	}
	if err = f.Close(); err != nil {
		c.Logger.Error(err)
		return err
	}
	return nil
}
