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
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"k8s.io/apimachinery/pkg/watch"
)

// HAProxyController is ingress controller
type HAProxyController struct {
	k8s            *K8s
	cfg            Configuration
	osArgs         utils.OSArgs
	Client         api.HAProxyClient
	HAProxyCfgDir  string
	eventChan      chan SyncDataEvent
	serverlessPods map[string]int
	Logger         utils.Logger
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
		c.Logger.Info("Running Controller out of K8s cluster")
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
	c.Logger.Trace("HAProxy config sync started")
	reload := false

	err := c.Client.APIStartTransaction()
	if err != nil {
		c.Logger.Error(err)
		return err
	}
	defer func() {
		c.Client.APIDisposeTransaction()
	}()

	restart, reload := c.handleGlobalAnnotations()

	reload = c.handleDefaultService() || reload

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
				reload = c.handlePath(namespace, ingress, &IngressRule{}, ingress.DefaultBackend) || reload
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

			//handle Ingress annotations
			if len(ingress.Rules) == 0 {
				c.Logger.Debugf("Ingress %s/%s: no rules defined", ingress.Namespace, ingress.Name)
				continue
			}
			c.handleRateLimiting(ingress)
			c.handleRequestCapture(ingress)
			c.handleRequestSetHdr(ingress)
			c.handleRequestPathRewrite(ingress)
			c.handleRequestSetHost(ingress)
			c.handleResponseSetHdr(ingress)
			c.handleBlacklisting(ingress)
			c.handleWhitelisting(ingress)
			c.handleHTTPRedirect(ingress)
		}
	}

	c.handleProxyProtocol()

	reload = c.handleDefaultCertificate(usedCerts) || reload

	reload = c.handleHTTPS(usedCerts) || reload

	reload = c.FrontendHTTPReqsRefresh() || reload

	reload = c.FrontendHTTPRspsRefresh() || reload

	reload = c.FrontendTCPreqsRefresh() || reload

	reload = c.cfg.MapFiles.Refresh() || reload

	reload = c.handleTCPServices() || reload

	reload = c.refreshBackendSwitching() || reload

	err = c.Client.APICommitTransaction()
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

	c.Logger.Trace("HAProxy config sync terminated")
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

	c.Client, err = api.Init(HAProxyCFG, "haproxy", "/var/run/haproxy-runtime-api.sock")
	if err != nil {
		c.Logger.Panic(err)
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
	result, err := c.Client.ExecuteRaw("show servers state")
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
