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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/haproxytech/config-parser/v3/params"
	"github.com/haproxytech/config-parser/v3/types"
	"github.com/haproxytech/models/v2"
	"k8s.io/apimachinery/pkg/watch"

	config "github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/route"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// HAProxyController is ingress controller
type HAProxyController struct {
	k8s               *K8s
	Store             store.K8s
	PublishService    *store.Service
	IngressClass      string
	EmptyIngressClass bool
	ready             bool
	Cfg               config.ControllerCfg
	osArgs            utils.OSArgs
	Client            api.HAProxyClient
	eventChan         chan SyncDataEvent
	UpdateHandlers    []UpdateHandler
}

// Wrapping a Native-Client transaction and commit it.
// Returning an error to let panic or log it upon the scenario.
func (c *HAProxyController) clientAPIClosure(fn func() error) (err error) {
	if err = c.Client.APIStartTransaction(); err != nil {
		return err
	}
	defer func() {
		c.Client.APIDisposeTransaction()
	}()
	if err = fn(); err != nil {
		return err
	}
	if err = c.Client.APICommitTransaction(); err != nil {
		return err
	}
	return nil
}

// Start initializes and runs HAProxyController
func (c *HAProxyController) Start(osArgs utils.OSArgs) {
	var k8s *K8s
	var err error

	c.osArgs = osArgs
	logger.SetLevel(osArgs.LogLevel.LogLevel)
	if err = c.Cfg.Init(); err != nil {
		logger.Panic(err)
	}
	c.haproxyInitialize()
	c.initHandlers()

	logger.Panic(c.clientAPIClosure(func() error {
		logger.Error(c.handleBinds())
		if osArgs.PprofEnabled {
			logger.Error(c.handlePprof())
		}
		return nil
	}))

	parts := strings.Split(osArgs.PublishService, "/")
	if len(parts) == 2 {
		c.PublishService = &store.Service{
			Namespace: parts[0],
			Name:      parts[1],
			Status:    EMPTY,
			Addresses: []string{},
		}
	}

	if osArgs.External {
		kubeconfig := filepath.Join(utils.HomeDir(), ".kube", "config")
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
		logger.Printf("Running on Kubernetes version: %s %s", k8sVersion.String(), k8sVersion.Platform)
	}

	c.eventChan = make(chan SyncDataEvent, watch.DefaultChanSize*6)
	go c.monitorChanges()
}

// Stop handles shutting down HAProxyController
func (c *HAProxyController) Stop() {
	logger.Infof("Stopping Ingress Controller")
	logger.Error(c.haproxyService("stop"))
}

// updateHAProxy is the control loop syncing HAProxy configuration
func (c *HAProxyController) updateHAProxy() {
	logger.Trace("HAProxy config sync started")

	err := c.Client.APIStartTransaction()
	if err != nil {
		logger.Error(err)
		return
	}
	defer func() {
		c.Client.APIDisposeTransaction()
	}()

	reload, restart := c.handleGlobalConfig()

	if route.CustomRoutes {
		logger.Error(route.RoutesReset(c.Client))
		route.CustomRoutes = false
	}

	for _, namespace := range c.Store.Namespaces {
		if !namespace.Relevant {
			continue
		}
		for _, ingress := range namespace.Ingresses {
			if ingress.Status == DELETED {
				continue
			}
			if !c.igClassIsSupported(ingress) {
				logger.Debugf("ingress '%s/%s' ignored: no matching IngressClass", ingress.Namespace, ingress.Name)
				continue
			}
			if c.PublishService != nil {
				logger.Error(c.k8s.UpdateIngressStatus(ingress, c.PublishService))
			}
			if ingress.DefaultBackend != nil {
				if r, errSvc := c.setDefaultService(ingress, []string{c.Cfg.FrontHTTP, c.Cfg.FrontHTTPS}); errSvc != nil {
					logger.Errorf("Ingress '%s/%s': default backend: %s", ingress.Namespace, ingress.Name, errSvc)
				} else {
					reload = reload || r
				}
			}
			// Ingress secrets
			logger.Tracef("ingress '%s/%s': processing secrets...", ingress.Namespace, ingress.Name)
			for _, tls := range ingress.TLS {
				if tls.Status == store.DELETED {
					continue
				}
				crt, updated, _ := c.Cfg.Certificates.HandleTLSSecret(c.Store, haproxy.SecretCtx{
					DefaultNS:  ingress.Namespace,
					SecretPath: tls.SecretName.Value,
					SecretType: haproxy.FT_CERT,
				})
				if crt != "" && updated {
					reload = true
					logger.Debugf("Secret '%s' in ingress '%s/%s' was updated, reload required", tls.SecretName.Value, ingress.Namespace, ingress.Name)
				}
			}
			// Ingress annotations
			logger.Tracef("ingress '%s/%s': processing annotations...", ingress.Namespace, ingress.Name)
			if len(ingress.Rules) == 0 {
				logger.Debugf("Ingress %s/%s: no rules defined", ingress.Namespace, ingress.Name)
				continue
			}
			c.handleIngressAnnotations(ingress)
			// Ingress rules
			logger.Tracef("ingress '%s/%s': processing rules...", ingress.Namespace, ingress.Name)
			for _, rule := range ingress.Rules {
				for _, path := range rule.Paths {
					if r, errIng := c.handleIngressPath(ingress, rule.Host, path); errIng != nil {
						logger.Errorf("Ingress '%s/%s': %s", ingress.Namespace, ingress.Name, errIng)
					} else {
						reload = reload || r
					}
				}
			}
		}
	}

	for _, handler := range c.UpdateHandlers {
		r, errHandler := handler.Update(c.Store, &c.Cfg, c.Client)
		logger.Error(errHandler)
		reload = reload || r
	}

	err = c.Client.APICommitTransaction()
	if err != nil {
		logger.Error("unable to Sync HAProxy configuration !!")
		logger.Error(err)
		c.clean(true)
		return
	}
	c.clean(false)
	if !c.ready {
		c.setToReady()
	}
	switch {
	case restart:
		if err = c.haproxyService("restart"); err != nil {
			logger.Error(err)
		} else {
			logger.Info("HAProxy restarted")
		}
	case reload:
		if err = c.haproxyService("reload"); err != nil {
			logger.Error(err)
		} else {
			logger.Info("HAProxy reloaded")
		}
	}

	logger.Trace("HAProxy config sync ended")
}

// setToRready exposes readiness endpoint
func (c *HAProxyController) setToReady() {
	logger.Panic(c.clientAPIClosure(func() error {
		return c.Client.FrontendBindEdit("healthz",
			models.Bind{
				Name:    "v4",
				Address: "0.0.0.0:1042",
			})
	}))
	if !c.osArgs.DisableIPV6 {
		logger.Panic(c.clientAPIClosure(func() error {
			return c.Client.FrontendBindCreate("healthz",
				models.Bind{
					Name:    "v6",
					Address: ":::1042",
					V4v6:    true,
				})
		}))
	}
	logger.Debugf("healthz frontend exposed for readiness probe")
	cm := c.Store.ConfigMaps.Main
	if cm.Name != "" && !cm.Loaded {
		logger.Warningf("Main configmap '%s/%s' not found", cm.Namespace, cm.Name)
	}
	c.ready = true
}

// haproxyInitialize initializes HAProxy environment and its API client.
func (c *HAProxyController) haproxyInitialize() {
	var err error

	// Initialize HAProxy client API
	c.Client, err = api.Init(c.Cfg.Env.TransactionDir, c.Cfg.Env.MainCFGFile, c.Cfg.Env.HAProxyBinary, c.Cfg.Env.RuntimeSocket)
	if err != nil {
		logger.Panic(err)
	}
	if c.osArgs.External && !c.osArgs.Test {
		logger.Panic(c.clientAPIClosure(func() error {
			var errors utils.Errors
			errors.Add(
				// Configure runtime socket
				c.Client.RuntimeSocket(nil),
				c.Client.RuntimeSocket(&types.Socket{
					Path: c.Cfg.Env.RuntimeSocket,
					Params: []params.BindOption{
						&params.BindOptionDoubleWord{Name: "expose-fd", Value: "listeners"},
						&params.BindOptionValue{Name: "level", Value: "admin"},
					},
				}),
				// Configure pidfile
				c.Client.PIDFile(&types.StringC{Value: c.Cfg.Env.PIDFile}),
				// Configure server-state-base
				c.Client.ServerStateBase(&types.StringC{Value: c.Cfg.Env.StateDir}),
			)
			return errors.Result()
		}))
	}

	//nolint:gosec //checks on HAProxyBinary should be done in configuration module.
	cmd := exec.Command(c.Cfg.Env.HAProxyBinary, "-v")
	haproxyInfo, err := cmd.Output()
	if err == nil {
		haproxyInfo := strings.Split(string(haproxyInfo), "\n")
		logger.Printf("Running with %s", haproxyInfo[0])
	} else {
		logger.Error(err)
	}

	logger.Printf("Starting HAProxy with %s", c.Cfg.Env.MainCFGFile)
	logger.Panic(c.haproxyService("start"))

	hostname, err := os.Hostname()
	logger.Error(err)
	logger.Printf("Running on %s", hostname)
}

// handleBind configures Frontends bind lines
func (c *HAProxyController) handleBinds() (err error) {
	var errors utils.Errors
	frontends := make(map[string]int64, 2)
	protos := make(map[string]string, 2)
	if !c.osArgs.DisableHTTP {
		frontends[c.Cfg.FrontHTTP] = c.osArgs.HTTPBindPort
	}
	if !c.osArgs.DisableHTTPS {
		frontends[c.Cfg.FrontHTTPS] = c.osArgs.HTTPSBindPort
	}
	if !c.osArgs.DisableIPV4 {
		protos["v4"] = c.osArgs.IPV4BindAddr
	}
	if !c.osArgs.DisableIPV6 {
		protos["v6"] = c.osArgs.IPV6BindAddr

		// IPv6 not disabled, so add v6 listening to stats frontend
		errors.Add(c.Client.FrontendBindCreate("stats",
			models.Bind{
				Name:    "v6",
				Address: ":::1024",
				V4v6:    false,
			}))
	}
	for ftName, ftPort := range frontends {
		for proto, addr := range protos {
			bind := models.Bind{
				Name:    proto,
				Address: addr,
				Port:    utils.PtrInt64(ftPort),
			}
			if err = c.Client.FrontendBindEdit(ftName, bind); err != nil {
				errors.Add(c.Client.FrontendBindCreate(ftName, bind))
			}
		}
	}
	return errors.Result()
}

// handlePprof enables  pprof backend
func (c *HAProxyController) handlePprof() (err error) {
	pprofBackend := "pprof"

	err = c.Client.BackendCreate(models.Backend{
		Name: pprofBackend,
		Mode: "http",
	})
	if err != nil {
		return err
	}
	err = c.Client.BackendServerCreate(pprofBackend, models.Server{
		Name:    "pprof",
		Address: "127.0.0.1:6060",
	})
	if err != nil {
		return err
	}
	logger.Debug("pprof backend created")
	err = route.AddHostPathRoute(route.Route{
		BackendName: pprofBackend,
		Path: &store.IngressPath{
			Path:           "/debug/pprof",
			ExactPathMatch: false,
		},
	}, c.Cfg.MapFiles)
	if err != nil {
		return err
	}
	c.Cfg.ActiveBackends[pprofBackend] = struct{}{}
	return nil
}

// clean controller state
func (c *HAProxyController) clean(failedSync bool) {
	logger.Error(c.Cfg.Clean())
	if c.PublishService != nil {
		c.PublishService.Status = EMPTY
	}
	c.Cfg.SSLPassthrough = false
	if !failedSync {
		c.Store.Clean()
	}
}
