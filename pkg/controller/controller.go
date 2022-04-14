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
	"path/filepath"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/google/renameio"

	"github.com/haproxytech/client-native/v2/models"
	config "github.com/haproxytech/kubernetes-ingress/pkg/configuration"
	"github.com/haproxytech/kubernetes-ingress/pkg/controller/route"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/process"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/apimachinery/pkg/watch"
)

var logger = utils.GetLogger()

// HAProxyController is ingress controller
type HAProxyController struct {
	cfg            config.ControllerCfg
	client         api.HAProxyClient
	osArgs         utils.OSArgs
	store          store.K8s
	publishService *utils.NamespaceValue
	auxCfgModTime  int64
	eventChan      chan k8s.SyncDataEvent
	ingressChan    chan ingress.Sync
	k8s            k8s.K8s
	ready          bool
	reload         bool
	restart        bool
	updateHandlers []UpdateHandler
	haproxyProcess process.Process
	podNamespace   string
	podPrefix      string
}

// Wrapping a Native-Client transaction and commit it.
// Returning an error to let panic or log it upon the scenario.
func (c *HAProxyController) clientAPIClosure(fn func() error) (err error) {
	if err = c.client.APIStartTransaction(); err != nil {
		return err
	}
	defer func() {
		c.client.APIDisposeTransaction()
	}()
	if err = fn(); err != nil {
		return err
	}

	if err = c.client.APICommitTransaction(); err != nil {
		return err
	}
	return nil
}

// Start initializes and runs HAProxyController
func (c *HAProxyController) Start(haproxyConf []byte) {
	var err error
	logger.SetLevel(c.osArgs.LogLevel.LogLevel)
	err = renameio.WriteFile(c.cfg.Env.MainCFGFile, haproxyConf, 0755)
	if err != nil {
		logger.Panic(err)
	}
	logger.Error(os.Chdir(c.cfg.Env.CfgDir))

	// Initialize controller
	err = c.cfg.Init()
	if err != nil {
		logger.Panic(err)
	}

	c.client, err = api.Init(c.cfg.Env.CfgDir, c.cfg.Env.MainCFGFile, c.cfg.Env.HAProxyBinary, c.cfg.Env.RuntimeSocket)
	if err != nil {
		logger.Panic(err)
	}

	c.initHandlers()
	c.haproxyStartup()

	// Controller PublishService
	parts := strings.Split(c.osArgs.PublishService, "/")
	if len(parts) == 2 {
		c.publishService = &utils.NamespaceValue{
			Namespace: parts[0],
			Name:      parts[1],
		}
	}

	// Get K8s client
	var restConfig *rest.Config
	if c.osArgs.External {
		kubeconfig := filepath.Join(utils.HomeDir(), ".kube", "config")
		if c.osArgs.KubeConfig != "" {
			kubeconfig = c.osArgs.KubeConfig
		}
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		restConfig, err = rest.InClusterConfig()
	}
	logger.Panicf("Unable to get kubernetes client config: %s", err)

	c.k8s = k8s.New(restConfig, c.osArgs, c.eventChan)
	x := c.k8s.GetClient().Discovery()
	if k8sVersion, err := x.ServerVersion(); err != nil {
		logger.Panicf("Unable to get Kubernetes version: %v\n", err)
	} else {
		logger.Printf("Running on Kubernetes version: %s %s", k8sVersion.String(), k8sVersion.Platform)
	}

	// Monitor k8s events
	var chanSize int64 = int64(watch.DefaultChanSize * 6)
	if c.osArgs.ChannelSize > 0 {
		chanSize = c.osArgs.ChannelSize
	}
	logger.Infof("Channel size: %d", chanSize)
	c.eventChan = make(chan k8s.SyncDataEvent, chanSize)
	go c.monitorChanges()
	if c.publishService != nil {
		// Update Ingress status
		c.ingressChan = make(chan ingress.Sync, chanSize)
		go ingress.UpdateStatus(c.k8s.GetClient(), c.store, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, c.ingressChan)
	}
}

// Stop handles shutting down HAProxyController
func (c *HAProxyController) Stop() {
	logger.Infof("Stopping Ingress Controller")
	logger.Error(c.haproxyService("stop"))
}

// updateHAProxy is the control loop syncing HAProxy configuration
func (c *HAProxyController) updateHAProxy() {
	var reload bool
	var err error
	logger.Trace("HAProxy config sync started")

	err = c.client.APIStartTransaction()
	if err != nil {
		logger.Error(err)
		return
	}
	defer func() {
		c.client.APIDisposeTransaction()
	}()

	reload, restart := c.handleGlobalConfig()
	c.reload = c.reload || reload
	c.restart = c.restart || restart

	if len(route.CustomRoutes) != 0 {
		logger.Error(route.CustomRoutesReset(c.client))
	}

	for _, namespace := range c.store.Namespaces {
		if !namespace.Relevant {
			continue
		}
		for _, ingResource := range namespace.Ingresses {
			i := ingress.New(c.store, ingResource, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass)
			if i == nil {
				logger.Debugf("ingress '%s/%s' ignored: no matching IngressClass", ingResource.Namespace, ingResource.Name)
				continue
			}
			if c.publishService != nil && ingResource.Status == store.ADDED {
				select {
				case c.ingressChan <- ingress.Sync{Ingress: ingResource}:
				default:
					logger.Errorf("Ingress %s/%s: unable to sync status: sync channel full", ingResource.Namespace, ingResource.Name)
				}
			}
			c.reload = i.Update(c.store, &c.cfg, c.client) || c.reload
		}
	}

	for _, handler := range c.updateHandlers {
		reload, err = handler.Update(c.store, &c.cfg, c.client)
		logger.Error(err)
		c.reload = c.reload || reload
	}

	err = c.client.APICommitTransaction()
	if err != nil {
		logger.Error("unable to Sync HAProxy configuration !!")
		logger.Error(err)
		c.clean(true)
		return
	}

	if !c.ready {
		c.setToReady()
	}

	switch {
	case c.restart:
		if err = c.haproxyService("restart"); err != nil {
			logger.Error(err)
		} else {
			logger.Info("HAProxy restarted")
		}
	case c.reload:
		if err = c.haproxyService("reload"); err != nil {
			logger.Error(err)
		} else {
			logger.Info("HAProxy reloaded")
		}
	}

	c.clean(false)

	logger.Trace("HAProxy config sync ended")
}

// setToRready exposes readiness endpoint
func (c *HAProxyController) setToReady() {
	logger.Panic(c.clientAPIClosure(func() error {
		return c.client.FrontendBindEdit("healthz",
			models.Bind{
				Name:    "v4",
				Address: "0.0.0.0:1042",
			})
	}))
	if !c.osArgs.DisableIPV6 {
		logger.Panic(c.clientAPIClosure(func() error {
			return c.client.FrontendBindCreate("healthz",
				models.Bind{
					Name:    "v6",
					Address: ":::1042",
					V4v6:    true,
				})
		}))
	}
	logger.Debugf("healthz frontend exposed for readiness probe")
	cm := c.store.ConfigMaps.Main
	if cm.Name != "" && !cm.Loaded {
		logger.Warningf("Main configmap '%s/%s' not found", cm.Namespace, cm.Name)
	}
	c.ready = true
}

// clean controller state
func (c *HAProxyController) clean(failedSync bool) {
	logger.Error(c.cfg.Clean())
	c.cfg.SSLPassthrough = false
	if !failedSync {
		c.store.Clean()
	}
	c.reload = false
	c.restart = false
}
