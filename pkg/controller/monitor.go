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

	"k8s.io/apimachinery/pkg/labels"

	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// SyncData gets all kubernetes changes, aggregates them and apply to HAProxy.
// All the changes must come through this function
//
//revive:disable-next-line:cyclomatic,cognitive-complexity
func (c *HAProxyController) SyncData() {
	hadChanges := false
	for job := range c.eventChan {
		if c.sessions != nil && k8ssync.IsNamespacedSessionEvent(job.SyncType) &&
			!c.sessions.Accept(job.Namespace, job.NamespaceEpoch) {
			if job.EventProcessed != nil {
				close(job.EventProcessed)
			}
			continue
		}
		ns := c.store.GetNamespace(job.Namespace)
		change := false
		switch job.SyncType {
		case k8ssync.BARRIER:
			// Acknowledge prior store events without generating configuration.
			c.retryUnwatchedSelectedNamespaces()
		case k8ssync.COMMAND:
			c.retryUnwatchedSelectedNamespaces()
			c.auxCfgManager()
			// create a NeedAction function.
			if hadChanges || instance.NeedReload() {
				c.updateHAProxy()
				hadChanges = false
				if job.EventProcessed != nil {
					close(job.EventProcessed)
				}
				continue
			}
		case k8ssync.CR_GLOBAL:
			var data *v3.Global
			if job.Data != nil {
				//revive:disable-next-line:unchecked-type-assertion
				data = job.Data.(*v3.Global)
			}
			change = c.store.EventGlobalCR(job.Namespace, job.Name, data)
		case k8ssync.CR_DEFAULTS:
			var data *v3.Defaults
			if job.Data != nil {
				//revive:disable-next-line:unchecked-type-assertion
				data = job.Data.(*v3.Defaults)
			}
			change = c.store.EventDefaultsCR(job.Namespace, job.Name, data)
		case k8ssync.CR_BACKEND:
			var data *v3.Backend
			if job.Data != nil {
				//revive:disable-next-line:unchecked-type-assertion
				data = job.Data.(*v3.Backend)
			}
			change = c.store.EventBackendCR(job.Namespace, job.Name, data)
		case k8ssync.CR_FRONTEND:
			var data *v3.Frontend
			if job.Data != nil {
				//revive:disable-next-line:unchecked-type-assertion
				data = job.Data.(*v3.Frontend)
			}
			change = c.store.EventFrontendCR(job.Namespace, job.Name, data)
		case k8ssync.NAMESPACE:
			//revive:disable-next-line:unchecked-type-assertion
			nsData := job.Data.(*store.Namespace)
			if c.sessions != nil {
				change = c.syncSelectorNamespace(nsData)
				break
			}
			change = c.store.EventNamespace(ns, nsData)
		case k8ssync.NAMESPACE_SESSION_READY:
			if c.sessions != nil && !c.sessions.MarkReady(job.Namespace, job.NamespaceEpoch) {
				break
			}
			change = c.store.MarkNamespaceReady(job.Namespace)
			hadChanges = hadChanges || change
			if job.EventProcessed != nil {
				close(job.EventProcessed)
			}
			continue
		case k8ssync.NAMESPACE_WATCH_RETIRED:
			pending := c.store.TakePendingNamespace(job.Namespace)
			change = c.store.RetireNamespace(job.Namespace)
			if c.sessions != nil {
				c.sessions.FinishDrain(job.Namespace)
			}
			if pending != nil {
				change = c.store.EventNamespace(c.store.GetNamespace(pending.Name), pending) || change
				c.startNamespaceWatch(job.Namespace)
			}
		case k8ssync.INGRESS:
			//revive:disable-next-line:unchecked-type-assertion
			change = c.store.EventIngress(ns, job.Data.(*store.Ingress), job.UID, job.ResourceVersion)
		case k8ssync.INGRESS_CLASS:
			//revive:disable-next-line:unchecked-type-assertion
			change = c.store.EventIngressClass(job.Data.(*store.IngressClass))
		case k8ssync.ENDPOINTS:
			//revive:disable-next-line:unchecked-type-assertion
			ep := job.Data.(*store.Endpoints)
			if c.store.SkipNamespaceInConfig(ns) {
				_ = c.store.EventEndpoints(ns, ep, func(*store.RuntimeBackend) error { return nil })
				change = false
				break
			}
			change = c.store.EventEndpoints(ns, ep, c.haproxy.SyncBackendSrvs)
		case k8ssync.SERVICE:
			//revive:disable-next-line:unchecked-type-assertion
			change = c.store.EventService(ns, job.Data.(*store.Service))
		case k8ssync.CONFIGMAP:
			//revive:disable-next-line:unchecked-type-assertion
			change = c.store.EventConfigMap(ns, job.Data.(*store.ConfigMap))
		case k8ssync.SECRET:
			//revive:disable-next-line:unchecked-type-assertion
			change = c.store.EventSecret(ns, job.Data.(*store.Secret))
		case k8ssync.POD:
			//revive:disable-next-line:unchecked-type-assertion
			change = c.store.EventPod(job.Data.(store.PodEvent))
		case k8ssync.PUBLISH_SERVICE:
			//revive:disable-next-line:unchecked-type-assertion
			change = c.store.EventPublishService(ns, job.Data.(*store.Service))
		case k8ssync.GATEWAYCLASS:
			change = c.store.EventGatewayClass(job.Data.(*store.GatewayClass))
		case k8ssync.GATEWAY:
			change = c.store.EventGateway(ns, job.Data.(*store.Gateway))
		case k8ssync.TCPROUTE:
			change = c.store.EventTCPRoute(ns, job.Data.(*store.TCPRoute))
		case k8ssync.REFERENCEGRANT:
			change = c.store.EventReferenceGrant(ns, job.Data.(*store.ReferenceGrant))
		case k8ssync.CUSTOM_RESOURCE:
			change = true
		case k8ssync.CR_TCP:
			var data *store.TCPs
			if job.Data != nil {
				//revive:disable-next-line:unchecked-type-assertion
				data = job.Data.(*store.TCPs)
			}
			change = c.store.EventTCPCR(job.Namespace, job.Name, data)
		}
		// Dormant/Starting namespaces keep Store fresh but must not schedule
		// updateHAProxy: that path always rebuilds global/defaults and used to
		// reload HAProxy on every sync when the global diff never converged.
		if change && k8ssync.IsNamespacedSessionEvent(job.SyncType) && c.store.SkipNamespaceInConfig(ns) {
			change = false
		}
		hadChanges = hadChanges || change
		if job.EventProcessed != nil {
			close(job.EventProcessed)
		}
	}
}

func (c *HAProxyController) startNamespaceWatch(name string) {
	if c.sessions == nil {
		return
	}
	if err := c.sessions.Start(name); err != nil {
		logger.Errorf("namespace-label-selector: failed to start watch for namespace %s: %s", name, err)
	}
}

func (c *HAProxyController) syncSelectorNamespace(nsData *store.Namespace) bool {
	if nsData.Status == store.DELETED {
		c.store.ForgetPendingNamespace(nsData.Name)
		changed := c.store.SetNamespaceDormant(nsData.Name)
		if !c.sessions.Drain(nsData.Name) {
			changed = c.store.RetireNamespace(nsData.Name) || changed
		}
		return changed
	}
	// Watch vs Relevant are separate, as in whitelist mode: the --configmap
	// namespace is always watched, but only matching labels make it Relevant.
	labelMatch := c.store.NamespacesAccess.Selector != nil &&
		c.store.NamespacesAccess.Selector.Matches(labels.Set(nsData.Labels))
	watched := c.store.NamespaceAlwaysSelected(nsData.Name) || labelMatch
	if c.sessions.Draining(nsData.Name) {
		if watched {
			c.store.RememberPendingNamespace(nsData)
		} else {
			c.store.ForgetPendingNamespace(nsData.Name)
		}
		return false
	}
	if watched {
		if _, selected := c.store.NamespacesAccess.Selected[nsData.Name]; !selected {
			nsData.Status = store.ADDED
		}
		change := c.store.EventNamespace(c.store.GetNamespace(nsData.Name), nsData)
		c.startNamespaceWatch(nsData.Name)
		if !c.sessions.Ready(nsData.Name) {
			// Starting: keep the store current but do not rebuild HAProxy until
			// NAMESPACE_SESSION_READY. EventNamespace(ADDED) is always true.
			return false
		}
		if labelMatch {
			return c.store.MarkNamespaceReady(nsData.Name) || change
		}
		return c.store.SetNamespaceDormant(nsData.Name)
	}
	if _, selected := c.store.NamespacesAccess.Selected[nsData.Name]; !selected {
		return false
	}
	_ = c.store.EventNamespace(c.store.GetNamespace(nsData.Name), nsData)
	return c.store.SetNamespaceDormant(nsData.Name)
}

// retryUnwatchedSelectedNamespaces restarts resource watches for namespaces
// that are selected but have no live session. Start is a no-op when a
// session is already Starting; this recovers a failed Start without waiting
// for the next Namespace informer resync.
func (c *HAProxyController) retryUnwatchedSelectedNamespaces() {
	if c.sessions == nil {
		return
	}
	for name := range c.store.NamespacesAccess.Selected {
		if c.sessions.Draining(name) || c.sessions.Ready(name) {
			continue
		}
		c.startNamespaceWatch(name)
	}
}

func (c *HAProxyController) auxCfgManager() {
	info, errStat := os.Stat(c.haproxy.AuxCFGFile)
	var (
		modifTimeSec int64
		auxCfgFile   = c.haproxy.AuxCFGFile
		useAuxFile   bool
	)

	defer func() {
		// Nothing changed
		if c.auxCfgModTime == modifTimeSec {
			return
		}
		// Apply decisions
		c.haproxy.SetAuxCfgFile(auxCfgFile)
		c.haproxy.UseAuxFile(useAuxFile)
		// The file exists now  (modifTimeSec !=0 otherwise nothing changed case).
		instance.ReloadIf(c.auxCfgModTime == 0, "auxiliary configuration file created")
		instance.ReloadIf(c.auxCfgModTime != 0, "auxiliary configuration file modified")
		c.auxCfgModTime = modifTimeSec
		if c.auxCfgModTime != 0 {
			logger.Infof("Auxiliary HAProxy config '%s' updated", auxCfgFile)
		}
	}()

	// File does not exist
	if errStat != nil {
		// nullify it
		auxCfgFile = ""
		if c.auxCfgModTime == 0 {
			// never existed before
			return
		}
		instance.Reload("Auxiliary HAProxy config '%s' removed", c.haproxy.AuxCFGFile)
		return
	}
	// File exists
	useAuxFile = true
	modifTimeSec = info.ModTime().Unix()
}
