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
	"fmt"
	"os"

	"github.com/haproxytech/client-native/v3/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/haproxytech/kubernetes-ingress/pkg/route"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetLogger()

// HAProxyController is ingress controller
type HAProxyController struct {
	haproxy        haproxy.HAProxy
	osArgs         utils.OSArgs
	store          store.K8s
	annotations    annotations.Annotations
	publishService *utils.NamespaceValue
	auxCfgModTime  int64
	eventChan      chan k8s.SyncDataEvent
	ingressChan    chan ingress.Sync
	ready          bool
	reload         bool
	restart        bool
	updateHandlers []UpdateHandler
	podNamespace   string
	podPrefix      string
	chShutdown     chan struct{}
}

// Wrapping a Native-Client transaction and commit it.
// Returning an error to let panic or log it upon the scenario.
func (c *HAProxyController) clientAPIClosure(fn func() error) (err error) {
	if err = c.haproxy.APIStartTransaction(); err != nil {
		return err
	}
	defer func() {
		c.haproxy.APIDisposeTransaction()
	}()
	if err = fn(); err != nil {
		return err
	}

	if err = c.haproxy.APICommitTransaction(); err != nil {
		return err
	}
	return nil
}

// Start initializes and runs HAProxyController
func (c *HAProxyController) Start() {
	c.initHandlers()
	logger.Error(c.setupHAProxyRules())
	logger.Error(os.Chdir(c.haproxy.Env.CfgDir))
	logger.Panic(c.haproxy.Service("start"))

	c.SyncData()
}

// Stop handles shutting down HAProxyController
func (c *HAProxyController) Stop() {
	logger.Infof("Stopping Ingress Controller")
	close(c.chShutdown)
	logger.Error(c.haproxy.Service("stop"))
}

// updateHAProxy is the control loop syncing HAProxy configuration
func (c *HAProxyController) updateHAProxy() {
	var reload bool
	var err error
	logger.Trace("HAProxy config sync started")

	err = c.haproxy.APIStartTransaction()
	if err != nil {
		logger.Error(err)
		return
	}
	defer func() {
		c.haproxy.APIDisposeTransaction()
	}()

	reload, restart := c.handleGlobalConfig()
	c.reload = c.reload || reload
	c.restart = c.restart || restart

	if len(route.CustomRoutes) != 0 {
		logger.Error(route.CustomRoutesReset(c.haproxy))
	}

	for _, namespace := range c.store.Namespaces {
		if !namespace.Relevant {
			continue
		}
		for _, ingResource := range namespace.Ingresses {
			i := ingress.New(c.store, ingResource, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, c.annotations)
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
			c.reload = i.Update(c.store, c.haproxy, c.annotations) || c.reload
		}
	}

	for _, handler := range c.updateHandlers {
		reload, err = handler.Update(c.store, c.haproxy, c.annotations)
		logger.Error(err)
		c.reload = c.reload || reload
	}

	err = c.haproxy.APICommitTransaction()
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
		if err = c.haproxy.Service("restart"); err != nil {
			logger.Error(err)
		} else {
			logger.Info("HAProxy restarted")
		}
	case c.reload:
		if err = c.haproxy.Service("reload"); err != nil {
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
	healthzPort := c.osArgs.HealthzBindPort
	logger.Panic(c.clientAPIClosure(func() error {
		return c.haproxy.FrontendBindEdit("healthz",
			models.Bind{
				BindParams: models.BindParams{
					Name: "v4",
				},
				Address: fmt.Sprintf("0.0.0.0:%d", healthzPort),
			})
	}))
	if !c.osArgs.DisableIPV6 {
		logger.Panic(c.clientAPIClosure(func() error {
			return c.haproxy.FrontendBindCreate("healthz",
				models.Bind{
					BindParams: models.BindParams{
						Name: "v6",
						V4v6: true,
					},
					Address: fmt.Sprintf(":::%d", healthzPort),
				})
		}))
	}

	logger.Panic(c.clientAPIClosure(func() error {
		ip := "127.0.0.1"
		return c.haproxy.PeerEntryEdit("localinstance",
			models.PeerEntry{
				Name:    "local",
				Address: &ip,
				Port:    &c.osArgs.LocalPeerPort,
			},
		)
	}))

	logger.Panic(c.clientAPIClosure(func() error {
		return c.haproxy.FrontendBindEdit("stats",
			models.Bind{
				BindParams: models.BindParams{
					Name: "stats",
				},
				Address: fmt.Sprintf("*:%d", c.osArgs.StatsBindPort),
			},
		)
	}))

	logger.Debugf("healthz frontend exposed for readiness probe")
	cm := c.store.ConfigMaps.Main
	if cm.Name != "" && !cm.Loaded {
		logger.Warningf("Main configmap '%s/%s' not found", cm.Namespace, cm.Name)
	}
	c.ready = true
}

// setupHAProxyRules configures haproxy rules (set-var) required for the controller logic implementation
func (c *HAProxyController) setupHAProxyRules() error {
	var errors utils.Errors
	errors.Add(
		// ForwardedProto rule
		c.haproxy.AddRule(c.haproxy.FrontHTTPS, rules.SetHdr{
			ForwardedProto: true,
		}, false),
	)
	for _, frontend := range []string{c.haproxy.FrontHTTP, c.haproxy.FrontHTTPS} {
		errors.Add(
			// txn.base var used for logging
			c.haproxy.AddRule(frontend, rules.ReqSetVar{
				Name:       "base",
				Scope:      "txn",
				Expression: "base",
			}, false),
			// Backend switching rules.
			c.haproxy.AddRule(frontend, rules.ReqSetVar{
				Name:       "path",
				Scope:      "txn",
				Expression: "path",
			}, false),
			c.haproxy.AddRule(frontend, rules.ReqSetVar{
				Name:       "host",
				Scope:      "txn",
				Expression: "req.hdr(Host),field(1,:),lower",
			}, false),
			c.haproxy.AddRule(frontend, rules.ReqSetVar{
				Name:       "host_match",
				Scope:      "txn",
				Expression: fmt.Sprintf("var(txn.host),map(%s)", maps.GetPath(route.HOST)),
			}, false),
			c.haproxy.AddRule(frontend, rules.ReqSetVar{
				Name:       "host_match",
				Scope:      "txn",
				Expression: fmt.Sprintf("var(txn.host),regsub(^[^.]*,,),map(%s,'')", maps.GetPath(route.HOST)),
				CondTest:   "!{ var(txn.host_match) -m found }",
			}, false),
			c.haproxy.AddRule(frontend, rules.ReqSetVar{
				Name:       "path_match",
				Scope:      "txn",
				Expression: fmt.Sprintf("var(txn.host_match),concat(,txn.path,),map(%s)", maps.GetPath(route.PATH_EXACT)),
			}, false),
			c.haproxy.AddRule(frontend, rules.ReqSetVar{
				Name:       "path_match",
				Scope:      "txn",
				Expression: fmt.Sprintf("var(txn.host_match),concat(,txn.path,),map_beg(%s)", maps.GetPath(route.PATH_PREFIX)),
				CondTest:   "!{ var(txn.path_match) -m found }",
			}, false),
		)
	}
	return errors.Result()
}

// clean haproxy config state
func (c *HAProxyController) clean(failedSync bool) {
	c.haproxy.Clean()
	logger.Error(c.setupHAProxyRules())
	if !failedSync {
		c.store.Clean()
	}
	c.reload = false
	c.restart = false
}
