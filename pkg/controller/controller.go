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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-test/deep"

	maps0 "maps"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/fs"
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
	store                    store.K8s
	prometheusMetricsManager metrics.PrometheusMetricsManager
	gatewayManager           gateway.GatewayManager
	annotations              annotations.Annotations
	updateStatusManager      status.UpdateStatusManager
	eventChan                chan k8ssync.SyncDataEvent
	updatePublishServiceFunc func(ingresses []*ingress.Ingress, publishServiceAddresses []string)
	chShutdown               chan struct{}
	podNamespace             string
	podPrefix                string
	PodIP                    string
	Hostname                 string
	updateHandlers           []UpdateHandler
	beforeUpdateHandlers     []UpdateHandler
	haproxy                  haproxy.HAProxy
	osArgs                   utils.OSArgs
	auxCfgModTime            int64
	ready                    bool
	processIngress           func()
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
		err := c.haproxy.PeerEntryDelete("localinstance", "local")
		if err != nil {
			return err
		}
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
	_, errStart := (c.haproxy.Service("start"))
	logger.Panic(errStart)

	c.SyncData()
}

// Stop handles shutting down HAProxyController
func (c *HAProxyController) Stop() {
	logger.Infof("Stopping Ingress Controller")
	close(c.chShutdown)
	_, errStop := c.haproxy.Service("stop")
	logger.Error(errStop)
}

// updateHAProxy is the control loop syncing HAProxy configuration
func (c *HAProxyController) updateHAProxy() {
	var err error
	logger.Trace("HAProxy config sync started")
	c.prometheusMetricsManager.UnsetUnableSyncGauge()

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

	c.processIngress()

	updated := deep.Equal(route.CurentCustomRoutes, route.CustomRoutes, deep.FLAG_IGNORE_SLICE_ORDER)
	if len(updated) != 0 {
		route.CurentCustomRoutes = route.CustomRoutes
		instance.Reload("Custom Routes changed: %s", strings.Join(updated, "\n"))
	}

	c.gatewayManager.ManageGateway()

	for _, handler := range c.updateHandlers {
		logger.Error(handler.Update(c.store, c.haproxy, c.annotations))
	}

	fs.Writer.WaitUntilWritesDone()

	if !c.ready {
		c.setToReady()
	}

	err = c.haproxy.APIFinalCommitTransaction()
	if err != nil {
		c.prometheusMetricsManager.SetUnableSyncGauge()
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
		// If any error not from config snippet then pop the previous state of backends
		logger.Error(c.haproxy.PopPreviousBackends())
		return
	}

	if instance.NeedReload() {
		fs.RunDelayedFuncs()
		var msg string
		if msg, err = c.haproxy.Service("reload"); err != nil {
			logger.Error(err)
			errLines := strings.Split(msg, "\n")
			msg := ""
			// Extract only lines with [ALERT] prefix to reuse functions
			for _, line := range errLines {
				if strings.HasPrefix(line, "[ALERT]") {
					msg += strings.TrimPrefix(line, "[ALERT]") + "\n"
				}
			}

			c.prometheusMetricsManager.SetUnableSyncGauge()
			rerun, errCfgSnippet := annotations.CheckBackendConfigSnippetErrorOnReload(errors.New(msg), c.haproxy.Env.CfgDir)
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
			// If any error not from config snippet then pop the previous state of backends
			logger.Error(c.haproxy.PopPreviousBackends())
		} else {
			logger.Info("HAProxy reloaded")
		}
		c.prometheusMetricsManager.UpdateReloadMetrics(err)
	} else if c.osArgs.DisableDelayedWritingOnlyIfReload {
		// If the osArgs flag is set, then write the files to disk even if there is no reload of haproxy
		fs.RunDelayedFuncs()
	}

	c.clean(false)
	// If transaction succeeds thenpush backends state for any future recover.
	logger.Error(c.haproxy.PushPreviousBackends())
	logger.Trace("HAProxy config sync ended")
}

// setToRready exposes readiness endpoint
func (c *HAProxyController) setToReady() {
	healthzPort := c.osArgs.HealthzBindPort
	logger.Panic(c.haproxy.FrontendBindCreate("healthz",
		models.Bind{
			BindParams: models.BindParams{
				Name:   "v4",
				Thread: c.osArgs.HealthzBindThread,
			},
			Address: fmt.Sprintf("0.0.0.0:%d", healthzPort),
		}))
	if !c.osArgs.DisableIPV6 {
		logger.Panic(c.haproxy.FrontendBindCreate("healthz",
			models.Bind{
				BindParams: models.BindParams{
					Name:   "v6",
					V4v6:   true,
					Thread: c.osArgs.HealthzBindThread,
				},
				Address: fmt.Sprintf(":::%d", healthzPort),
			}))
	}

	logger.Panic(c.haproxy.FrontendBindCreate("stats",
		models.Bind{
			BindParams: models.BindParams{
				Name:   "stats",
				Thread: c.osArgs.StatsBindThread,
			},
			Address: fmt.Sprintf("*:%d", c.osArgs.StatsBindPort),
		},
	))

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
				Expression: fmt.Sprintf("var(txn.host_match),concat(,txn.path,),map(%s)", maps.GetPath(route.PATH_PREFIX_EXACT)),
				CondTest:   "!{ var(txn.path_match) -m found }",
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

func (c *HAProxyController) manageIngress(ing *store.Ingress) {
	i := ingress.New(ing, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, c.annotations)
	if !i.Supported(c.store, c.annotations) {
		logger.Debugf("ingress '%s/%s' ignored: no matching", ing.Namespace, ing.Name)
	} else {
		i.Update(c.store, c.haproxy, c.annotations)
	}
	if ing.Status == store.ADDED || ing.ClassUpdated {
		c.updateStatusManager.AddIngress(i)
	}
}

func (c *HAProxyController) processIngressesWithMerge() {
	for _, namespace := range c.store.Namespaces {
		c.store.SecretsProcessed = map[string]struct{}{}
		// Iterate over services
		for _, service := range namespace.Services {
			ingressesOrderedList := c.store.IngressesByService[service.Namespace+"/"+service.Name]
			if ingressesOrderedList == nil {
				continue
			}
			ingresses := ingressesOrderedList.Items()
			if len(ingresses) == 0 {
				continue
			}
			// Put standalone ingresses aside.
			var standaloneIngresses []*store.Ingress
			// Get the name of ingresses referring to the service
			var ingressesToMerge []*store.Ingress
			for _, ing := range ingresses {
				i := ingress.New(ing, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, c.annotations)
				if !i.Supported(c.store, c.annotations) {
					continue
				}
				// if the ingress has standalone-backend annotation, put it aside and continue.
				if ing.Annotations["standalone-backend"] == "true" {
					standaloneIngresses = append(standaloneIngresses, ing)
					continue
				}
				ingressesToMerge = append(ingressesToMerge, ing)
			}

			// Get copy of annotationsFromAllIngresses from all ingresses
			annotationsFromAllIngresses := map[string]string{}

			for _, ingressToMerge := range ingressesToMerge {
				// Gather all annotations from all ingresses referring to the service in a consistent order based on ingress name.
				for ann, value := range ingressToMerge.Annotations {
					if _, specific := annotations.SpecificAnnotations[ann]; specific {
						continue
					}
					annotationsFromAllIngresses[ann] = value
				}
			}

			// Now we've gathered the annotations set we can process all ingresses.
			for _, ingressToMerge := range ingressesToMerge {
				// We copy the ingress
				consolidatedIngress := *ingressToMerge
				// We assign the general set of annotations
				consolidatedIngressAnns := map[string]string{}
				maps0.Copy(consolidatedIngressAnns, annotationsFromAllIngresses)

				consolidatedIngress.Annotations = consolidatedIngressAnns
				for ann, value := range ingressToMerge.Annotations {
					if _, specific := annotations.SpecificAnnotations[ann]; !specific {
						continue
					}
					consolidatedIngress.Annotations[ann] = value
				}
				// We will reprocess the rules because we need to skip the ones referring to an other service.
				rules := map[string]*store.IngressRule{}
				consolidatedIngress.Rules = rules
				for _, rule := range ingressToMerge.Rules {
					newRule := store.IngressRule{
						Host:  rule.Host,
						Paths: map[string]*store.IngressPath{},
					}
					for _, path := range rule.Paths {
						// if the rule refers to the service then keep it ...
						if path.SvcNamespace == service.Namespace && path.SvcName == service.Name {
							newRule.Paths[path.Path] = path
						}
					}
					// .. if it's not empty
					if len(newRule.Paths) > 0 {
						rules[newRule.Host] = &newRule
					}
				}
				// Back to the usual processing of the ingress

				c.manageIngress(&consolidatedIngress)
			}
			// Now process the standalone ingresses as usual.
			for _, standaloneIngress := range standaloneIngresses {
				c.manageIngress(standaloneIngress)
			}
		}
	}
}

func (c *HAProxyController) processIngressesDefaultImplementation() {
	for _, namespace := range c.store.Namespaces {
		c.store.SecretsProcessed = map[string]struct{}{}
		for _, ingResource := range namespace.Ingresses {
			if !namespace.Relevant && !ingResource.Faked {
				// As we watch only for white-listed namespaces, we should not worry about iterating over
				// many ingresses in irrelevant namespaces.
				// There should only be fake ingresses in irrelevant namespaces so loop should be whithin small amount of ingresses (Prometheus)
				continue
			}
			c.manageIngress(ingResource)
		}
	}
}
