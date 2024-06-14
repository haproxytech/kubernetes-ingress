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
	"strings"

	"github.com/go-test/deep"

	"github.com/haproxytech/client-native/v5/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	gateway "github.com/haproxytech/kubernetes-ingress/pkg/gateways"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/metrics"
	"github.com/haproxytech/kubernetes-ingress/pkg/route"
	"github.com/haproxytech/kubernetes-ingress/pkg/status"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetLogger()

// HAProxyController is ingress controller
type HAProxyController struct {
	gatewayManager           gateway.GatewayManager
	annotations              annotations.Annotations
	eventChan                chan k8ssync.SyncDataEvent
	updatePublishServiceFunc func(ingresses []*ingress.Ingress, publishServiceAddresses []string)
	chShutdown               chan struct{}
	podNamespace             string
	podPrefix                string
	haproxy                  haproxy.HAProxy
	updateHandlers           []UpdateHandler
	store                    store.K8s
	osArgs                   utils.OSArgs
	auxCfgModTime            int64
	ready                    bool
	updateStatusManager      status.UpdateStatusManager
	beforeUpdateHandlers     []UpdateHandler
	prometheusMetricsManager metrics.PrometheusMetricsManager
	PodIP                    string
	Hostname                 string
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
	logger.Panic(c.clientAPIClosure(func() error {
		return c.haproxy.PeerEntryCreateOrEdit("localinstance",
			models.PeerEntry{
				Name:    c.Hostname,
				Address: &c.PodIP,
				Port:    &c.osArgs.LocalPeerPort,
			},
		)
	}))
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
	var err error
	logger.Trace("HAProxy config sync started")

	err = c.haproxy.APIStartTransaction()
	if err != nil {
		logger.Error(err)
		return
	}
	defer func() {
		c.haproxy.APIDisposeTransaction()
		instance.Reset()
	}()
	// First log here that will contain the "transactionID" field (added in APIStartTransaction)
	// All subsequent log line will contain the "transactionID" field.
	logger.Trace("HAProxy config sync transaction started")

	c.handleGlobalConfig()

	if len(route.CustomRoutes) != 0 {
		logger.Error(route.CustomRoutesReset(c.haproxy))
	}

	// global config-snippet
	logger.Error(annotations.NewCfgSnippet(
		annotations.ConfigSnippetOptions{
			Name:    "backend-config-snippet",
			Backend: utils.Ptr("configmap"),
			Ingress: nil,
		}).
		Process(c.store, c.store.ConfigMaps.Main.Annotations))

	for _, handler := range c.beforeUpdateHandlers {
		err = handler.Update(c.store, c.haproxy, c.annotations)
		logger.Error(err)
	}

	for _, namespace := range c.store.Namespaces {
		if !namespace.Relevant {
			continue
		}
		c.store.SecretsProcessed = map[string]struct{}{}
		for _, ingResource := range namespace.Ingresses {
			i := ingress.New(c.store, ingResource, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, c.annotations)
			if !i.Supported(c.store, c.annotations) {
				logger.Debugf("ingress '%s/%s' ignored: no matching", ingResource.Namespace, ingResource.Name)
			} else {
				i.Update(c.store, c.haproxy, c.annotations)
			}
			if ingResource.Status == store.ADDED || ingResource.ClassUpdated {
				c.updateStatusManager.AddIngress(i)
			}
		}
	}

	updated := deep.Equal(route.CurentCustomRoutes, route.CustomRoutes, deep.FLAG_IGNORE_SLICE_ORDER)
	if len(updated) != 0 {
		route.CurentCustomRoutes = route.CustomRoutes
		instance.Reload("Custom Routes changed: %s", strings.Join(updated, "\n"))
	}

	c.gatewayManager.ManageGateway()

	for _, handler := range c.updateHandlers {
		logger.Error(handler.Update(c.store, c.haproxy, c.annotations))
	}

	err = c.haproxy.APICommitTransaction()
	if err != nil {
		logger.Error("unable to Sync HAProxy configuration !!")
		logger.Error(err)
		rerun, errCfgSnippet := annotations.CheckBackendConfigSnippetError(err, c.haproxy.Env.CfgDir)
		logger.Error(errCfgSnippet)
		c.clean(true)
		if rerun {
			logger.Debug("disabling some config snippets because of errors")
			// We need to replay all these resources.
			c.store.SecretsProcessed = map[string]struct{}{}
			c.store.BackendsProcessed = map[string]struct{}{}
			c.updateHAProxy()
			return
		}
		return
	}

	if !c.ready {
		c.setToReady()
	}

	switch {
	case instance.NeedRestart():
		if err = c.haproxy.Service("restart"); err != nil {
			logger.Error(err)
		} else {
			logger.Info("HAProxy restarted")
		}
		c.prometheusMetricsManager.UpdateRestartMetrics(err)
	case instance.NeedReload():
		if err = c.haproxy.Service("reload"); err != nil {
			logger.Error(err)
		} else {
			logger.Info("HAProxy reloaded")
		}
		c.prometheusMetricsManager.UpdateReloadMetrics(err)
	}

	c.clean(false)

	logger.Trace("HAProxy config sync ended")
}

// setToRready exposes readiness endpoint
func (c *HAProxyController) setToReady() {
	healthzPort := c.osArgs.HealthzBindPort
	logger.Panic(c.clientAPIClosure(func() error {
		return c.haproxy.FrontendBindCreate("healthz",
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
		return c.haproxy.FrontendBindCreate("stats",
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
	var errs utils.Errors
	errs.Add(
		// ForwardedProto rule
		c.haproxy.AddRule(c.haproxy.FrontHTTPS, rules.SetHdr{
			ForwardedProto: true,
		}, false),
	)
	for _, frontend := range []string{c.haproxy.FrontHTTP, c.haproxy.FrontHTTPS} {
		errs.Add(
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
	return errs.Result()
}

// clean haproxy config state
func (c *HAProxyController) clean(failedSync bool) {
	c.haproxy.Clean()
	// Need to do that even if transaction failed otherwise at fix time, they won't be reprocessed.
	c.store.BackendsProcessed = map[string]struct{}{}
	logger.Error(c.setupHAProxyRules())
	if !failedSync {
		c.store.Clean()
	}
}

func (c *HAProxyController) SetGatewayAPIInstalled(gatewayAPIInstalled bool) {
	c.gatewayManager.SetGatewayAPIInstalled(gatewayAPIInstalled)
}
