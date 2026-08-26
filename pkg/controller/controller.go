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
	"slices"
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
	// defaultBackend holds, for the current reconciliation pass, the ingress
	// deterministically selected to provide the frontends' default backend.
	// Reset before each processIngress() run and consumed by setIngressDefaultBackend().
	defaultBackend *store.Ingress
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

	return c.haproxy.APICommitTransaction()
}

// Start initializes and runs HAProxyController
func (c *HAProxyController) Start() {
	logger.Panic(c.clientAPIClosure(func() error {
		err := c.haproxy.PeerEntryDelete("localinstance", "local")
		if err != nil {
			return err
		}
		return c.haproxy.PeerEntryCreateOrEdit(
			"localinstance",
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
	_, errStart := c.haproxy.Service("start")
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
		},
	).
		Process(c.store, c.store.ConfigMaps.Main.Annotations))

	for _, handler := range c.beforeUpdateHandlers {
		err = handler.Update(c.store, c.haproxy, c.annotations)
		logger.Error(err)
	}

	c.processSSLPassthroughInConfigFile()
	c.defaultBackend = nil
	c.processIngress()
	c.setIngressDefaultBackend()

	updated := deep.Equal(route.CurentCustomRoutes, route.CustomRoutes, deep.FLAG_IGNORE_SLICE_ORDER)
	if len(updated) != 0 {
		route.CurentCustomRoutes = route.CustomRoutes
		instance.Reload("Custom Routes changed: %s", strings.Join(updated, "\n"))
	}

	c.gatewayManager.ManageGateway()

	if !c.ready {
		c.setToReady()
	}

	for _, handler := range c.updateHandlers {
		logger.Error(handler.Update(c.store, c.haproxy, c.annotations))
	}

	fs.Writer.WaitUntilWritesDone()

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
			c.store.BackendsProcessed = map[string]store.BackendOwner{}
			c.store.RoutesProcessedByMapFile = map[string]map[string]store.RouteOwner{}
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
			c.prometheusMetricsManager.UpdateReloadMetrics(err)
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
				c.store.BackendsProcessed = map[string]store.BackendOwner{}
				c.store.RoutesProcessedByMapFile = map[string]map[string]store.RouteOwner{}
				c.updateHAProxy()
				return
			}
			// If any error not from config snippet then pop the previous state of backends
			logger.Error(c.haproxy.PopPreviousBackends())
		} else {
			logger.Info("HAProxy reloaded")
			c.prometheusMetricsManager.UpdateReloadMetrics(err)
		}
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

	logger.Panic(c.haproxy.FrontendBindCreate(
		"stats",
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
	c.store.BackendsProcessed = map[string]store.BackendOwner{}
	c.store.RoutesProcessedByMapFile = map[string]map[string]store.RouteOwner{}
	logger.Error(c.setupHAProxyRules())
	if !failedSync {
		c.store.Clean()
		annotations.Clean()
	}
}

func (c *HAProxyController) SetGatewayAPIInstalled(gatewayAPIInstalled bool) {
	c.gatewayManager.SetGatewayAPIInstalled(gatewayAPIInstalled)
}

func (c *HAProxyController) manageIngress(ing *store.Ingress) {
	i := ingress.New(ing, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, c.annotations)
	if !i.Admit(c.store) {
		logger.Debugf("ingress '%s/%s' ignored: no matching", ing.Namespace, ing.Name)
	} else {
		i.Update(c.store, c.haproxy, c.annotations)
		c.considerDefaultBackend(ing)
		if ing.Status == store.ADDED || ing.ClassUpdated {
			c.updateStatusManager.AddIngress(i)
		}
	}
}

//revive:disable-next-line:cognitive-complexity
func (c *HAProxyController) processIngressesWithMerge() {
	// Namespaces and services are walked by name for the same reason as in
	// processIngressesDefaultImplementation: rules are created in processing order and
	// RefreshRules replays them in that order, so a random walk produced a different
	// configuration text on every pass, hence a commit and a reload with nothing having
	// changed.
	//
	// The ingresses of a service need no sorting here: IngressesByService is an ordered
	// set, oldest first with the namespace and name settling equal creation times
	// (pkg/store/events.go), which is the order backend ownership needs. That same order
	// decides which annotation value wins the merge below, the first one found being kept,
	// so the established ingress has precedence there too. The two go together: reversing
	// the set without reversing the merge would give precedence to the newest ingress.
	for _, namespace := range sortedByKey(c.store.Namespaces) {
		c.store.SecretsProcessed = map[string]struct{}{}
		// Iterate over services
		for _, service := range sortedByKey(namespace.Services) {
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
				if !i.Admit(c.store) {
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
				// Gather all annotations from all ingresses referring to the service. The list
				// is ordered oldest first and the first value found is the one kept, so the
				// established ingress has precedence - the same rule as backend ownership.
				for ann, value := range ingressToMerge.Annotations {
					if _, specific := annotations.SpecificAnnotations[ann]; specific {
						continue
					}
					if _, alreadySet := annotationsFromAllIngresses[ann]; !alreadySet {
						annotationsFromAllIngresses[ann] = value
					}
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

// sortedByKey returns the values of m ordered by their key, so that walking a Go map
// — whose iteration order is randomized on every pass — becomes reproducible.
func sortedByKey[V any](m map[string]V) []V {
	out := make([]V, 0, len(m))
	for _, key := range slices.Sorted(maps0.Keys(m)) {
		out = append(out, m[key])
	}
	return out
}

// sortedIngresses returns the ingresses ordered by creation time, oldest first, and by
// name for those created within the same second — Kubernetes stores creationTimestamp
// with second granularity, so ingresses applied together commonly tie.
//
// Ownership of a shared backend goes to the first ingress processed, so ordering by age
// is what makes it belong to the oldest one. Ordering by name would only protect an
// established backend from newcomers whose name happens to sort after: one named to sort
// before would still take it over, reconfigure it and force a reload, which is precisely
// what ownership is meant to prevent.
//
// Ingresses sharing a backend are always in the same namespace, since an ingress can
// only reference services of its own namespace and the backend name carries that
// namespace. Ordering within a namespace is therefore enough, and the namespaces
// themselves can keep being walked by name.
func sortedIngresses(m map[string]*store.Ingress) []*store.Ingress {
	out := make([]*store.Ingress, 0, len(m))
	for _, ing := range m {
		out = append(out, ing)
	}
	slices.SortFunc(out, func(a, b *store.Ingress) int {
		if byAge := a.CreationTime.Compare(b.CreationTime); byAge != 0 {
			return byAge
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

func (c *HAProxyController) processIngressesDefaultImplementation() {
	// Namespaces are walked by name and ingresses by age. Several
	// decisions taken while walking depend on that order, the mode of a backend shared
	// by two ingresses being the sharpest one: a backend name derives from
	// (namespace, service, port name) and getBackendModel rebuilds its whole definition
	// from the annotations of the single ingress being processed, so the last one
	// processed wins. With map iteration order, that winner changed on every
	// reconciliation, and with it the backend mode, balance algorithm and options.
	// Sorting does not resolve the conflict — Ingress.claimBackendMode reports it — but
	// it makes the outcome reproducible and diagnosable.
	for _, namespace := range sortedByKey(c.store.Namespaces) {
		c.store.SecretsProcessed = map[string]struct{}{}
		for _, ingResource := range sortedIngresses(namespace.Ingresses) {
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

// processSSLPassthroughInConfigFile turns the ssl-passthrough topology on as soon as a
// single ingress needs it: the tcp ssl frontend is created and the https bind moves behind
// it. That decision has to be taken before the ingresses are processed, since the frontend
// a route is attached to depends on it, hence this separate pass.
func (c *HAProxyController) processSSLPassthroughInConfigFile() {
	for _, namespace := range c.store.Namespaces {
		for _, ingResource := range namespace.Ingresses {
			if !namespace.Relevant && !ingResource.Faked {
				// As we watch only for white-listed namespaces, we should not worry about iterating over
				// many ingresses in irrelevant namespaces.
				// There should only be fake ingresses in irrelevant namespaces so loop should be whithin small amount of ingresses (Prometheus)
				continue
			}
			// Supported and not Admit: this pass decides a global topology, it must not
			// record a class decision on the ingresses it walks.
			if !ingress.New(ingResource, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, c.annotations).Supported(c.store) {
				continue
			}
			if c.sslPassthroughRequested(ingResource) {
				haproxy.SSLPassthrough = true
				return
			}
		}
	}
}

// sslPassthroughRequested reports whether any traffic of this ingress is to be served in
// passthrough mode.
//
// The annotation being resolved against the service a path points at, the question is
// asked per path. An ingress declaring no rule has no path to ask about, so it is resolved
// from its own annotations and from the configmap - which keeps it able to turn the
// topology on, as it was before the service scope existed.
//
// A spec.defaultBackend is deliberately not consulted, although it does carry a service.
// Kubernetes allows it in place of the rules, so a rule-less ingress may well have one, but
// a default backend cannot be served in passthrough: it takes the mode of the frontends it
// is attached to, and passthrough routing is keyed on sni.map, which only carries host
// entries, while a default backend is what serves the requests no host matched. Consulting
// it would move the https bind of the whole controller for a backend no passthrough route
// can reach. reportPassthroughOnDefaultBackend warns about the annotation instead.
func (c *HAProxyController) sslPassthroughRequested(ingResource *store.Ingress) bool {
	report := func(err error) {
		logger.Errorf("Ingress '%s/%s': SSL Passthrough parsing: %s", ingResource.Namespace, ingResource.Name, err)
	}
	if len(ingResource.Rules) == 0 {
		enabled, err := annotations.Bool("ssl-passthrough", ingResource.Annotations, c.store.ConfigMaps.Main.Annotations)
		if err != nil {
			report(err)
		}
		return enabled
	}
	for _, rule := range ingResource.Rules {
		for _, path := range rule.Paths {
			enabled, err := ingress.SSLPassthroughEnabled(c.store, path, ingResource.Annotations)
			if err != nil {
				report(err)
			} else if enabled {
				return true
			}
		}
	}
	return false
}
