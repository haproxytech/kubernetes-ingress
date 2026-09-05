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

package k8s

import (
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/haproxytech/client-native/v6/models"
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	crinformersv1 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v1/informers/externalversions"
	crinformersv3 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v3/informers/externalversions"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	gatewaynetworking "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"
)

func (k *k8s) NewSessionManager(eventChan chan k8ssync.SyncDataEvent, gatewayAPI bool) NamespaceSessions {
	if k == nil || !k.osArgs.NamespaceLabelSelectorActive() {
		return nil
	}
	k.gatewayAPI = gatewayAPI
	// Assign k.sessions before taking the startNSSession method value.
	// A value-receiver method value copies the struct at bind time; binding
	// before this assignment made every Start fail with "session manager is nil".
	m := newSessionManager(eventChan, nil)
	k.sessions = m
	m.starter = k.startNSSession
	return m
}

func (k *k8s) startNSSession(namespace string, epoch uint64, stopCh chan struct{}) (*nsSession, error) {
	if k.sessions == nil {
		return nil, fmt.Errorf("session manager is nil")
	}
	eventChan := k.sessions.eventChan
	proxy := make(chan k8ssync.SyncDataEvent, 64)
	var stampWg sync.WaitGroup
	stampWg.Add(1)
	go func() {
		defer stampWg.Done()
		stampSessionEvents(proxy, eventChan, stopCh, epoch)
	}()

	var regs []cache.ResourceEventHandlerRegistration
	sk := *k
	sk.eventEpoch = epoch
	sk.handlerRegs = &regs

	sess := &nsSession{
		namespace: namespace,
		epoch:     epoch,
		stopCh:    stopCh,
	}

	core := k8sinformers.NewSharedInformerFactoryWithOptions(k.builtInClient, k.cacheResyncPeriod, k8sinformers.WithNamespace(namespace))
	sk.getServiceInformer(proxy, core)
	sk.getSecretInformer(proxy, core)
	ii, ici := sk.getIngressInformers(proxy, core, k.osArgs)
	if ii == nil {
		close(proxy)
		stampWg.Wait()
		return nil, fmt.Errorf("ingress resource not supported")
	}
	_ = ici
	epsi := sk.getEndpointSliceInformer(proxy, core)
	if epsi == nil || !k.endpointsMirroring() {
		sk.getEndpointsInformer(proxy, core)
	}

	crV1 := crinformersv1.NewSharedInformerFactoryWithOptions(k.crClientV1, k.cacheResyncPeriod, crinformersv1.WithNamespace(namespace))
	crV3 := crinformersv3.NewSharedInformerFactoryWithOptions(k.crClientV3, k.cacheResyncPeriod, crinformersv3.WithNamespace(namespace))

	var gw gatewaynetworking.SharedInformerFactory
	var gwWait sync.WaitGroup
	var gwRun []cache.SharedIndexInformer
	if k.gatewayAPI && k.gatewayClient != nil {
		gw = gatewaynetworking.NewSharedInformerFactoryWithOptions(k.gatewayClient, k.cacheResyncPeriod, gatewaynetworking.WithNamespace(namespace))
		for _, inf := range []cache.SharedIndexInformer{
			sk.getGatewayInformer(proxy, gw),
			sk.getTCPRouteInformer(proxy, gw),
			sk.getReferenceGrantInformer(proxy, gw),
		} {
			if inf == nil {
				continue
			}
			gwRun = append(gwRun, inf)
		}
	}

	sess.handlers = regs
	sess.crV1 = crV1
	sess.crV3 = crV3
	sess.gw = gw
	sess.proxy = proxy
	sess.run = func() {
		// The manager lock serializes this snapshot with late CR registration.
		// Construction may have overlapped CRD discovery while only a placeholder
		// was published; read the latest set now, not in the starter.
		crsV1, crsV3 := k.crsSnapshot()
		sk.runCRInformers(proxy, stopCh, namespace, &[]cache.InformerSynced{}, crsV1, crsV3, k.osArgs, false, crV1, crV3)
		sess.handlers = regs
		for _, inf := range gwRun {
			gwWait.Add(1)
			go func(inf cache.SharedIndexInformer) {
				defer gwWait.Done()
				inf.Run(stopCh)
			}(inf)
		}
		core.Start(stopCh)
		crV1.Start(stopCh)
		crV3.Start(stopCh)
	}
	sess.shutdown = func() {
		closeSessionProxyAfter(func() {
			core.Shutdown()
			crV1.Shutdown()
			crV3.Shutdown()
			gwWait.Wait()
		}, sess, &stampWg)
	}
	return sess, nil
}

func (k k8s) watchNamespacesByLabel(eventChan chan k8ssync.SyncDataEvent, stop chan struct{}, osArgs utils.OSArgs, gatewayAPIInstalled bool) {
	if k.sessions == nil {
		logger.Panic("namespace-label-selector is active but session manager is nil")
	}
	selector := osArgs.NamespaceLabelSelectorCanonical()
	informersSynced := &[]cache.InformerSynced{}

	k.runConfigMapInformers(eventChan, stop, informersSynced, osArgs.ConfigMap)
	k.runConfigMapInformers(eventChan, stop, informersSynced, osArgs.ConfigMapTCPServices)
	k.runConfigMapInformers(eventChan, stop, informersSynced, osArgs.ConfigMapErrorFiles)
	k.runConfigMapInformers(eventChan, stop, informersSynced, osArgs.ConfigMapPatternFiles)
	k.runProcessLevelIngressClass(eventChan, stop, informersSynced)
	if gatewayAPIInstalled {
		k.runProcessLevelGatewayClass(eventChan, stop, informersSynced)
	}

	nsFactory := k8sinformers.NewSharedInformerFactoryWithOptions(k.builtInClient, k.cacheResyncPeriod,
		k8sinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = selector
		}))
	var nsRegs []cache.ResourceEventHandlerRegistration
	k.handlerRegs = &nsRegs
	nsInformer := k.getSelectorNamespaceInformer(eventChan, nsFactory)
	k.handlerRegs = nil
	nsFactory.Start(stop)
	synced := []cache.InformerSynced{nsInformer.HasSynced}
	for _, reg := range nsRegs {
		if reg != nil {
			synced = append(synced, reg.HasSynced)
		}
	}
	if !cache.WaitForCacheSync(stop, synced...) {
		logger.Panic("Caches are not populated due to an underlying error, cannot run the Ingress Controller")
	}

	// Handler sync means all initial namespace events have been sent, not
	// processed. Wait for their Start/Stop calls in SyncData before checking
	// session readiness. Unlike COMMAND, this barrier does not publish a
	// configuration while the initial resource caches are still warming up.
	if !waitForNamespaceEvents(eventChan, stop) {
		return
	}
	if !k.sessions.WaitAllReady(stop) {
		logger.Panic("Caches are not populated due to an underlying error, cannot run the Ingress Controller")
	}

	k.RunCRSCreationMonitoring(eventChan, stop, osArgs)

	if !cache.WaitForCacheSync(stop, *informersSynced...) {
		logger.Panic("Caches are not populated due to an underlying error, cannot run the Ingress Controller")
	}

	syncPeriod := k.syncPeriod
	initialSyncPeriod := k.initialSyncPeriod
	logger.Debugf("Executing first transaction after %s", initialSyncPeriod.String())
	logger.Debugf("Executing new transaction every %s", syncPeriod.String())
	time.Sleep(k.initialSyncPeriod)
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND}
	for {
		time.Sleep(syncPeriod)
		ep := make(chan struct{})
		eventChan <- k8ssync.SyncDataEvent{
			SyncType:       k8ssync.COMMAND,
			EventProcessed: ep,
		}
		<-ep
	}
}

func waitForNamespaceEvents(eventChan chan<- k8ssync.SyncDataEvent, stop <-chan struct{}) bool {
	processed := make(chan struct{})
	select {
	case <-stop:
		return false
	case eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.BARRIER, EventProcessed: processed}:
	}
	select {
	case <-stop:
		return false
	case <-processed:
		return true
	}
}

func (k k8s) runProcessLevelIngressClass(eventChan chan k8ssync.SyncDataEvent, stop chan struct{}, informersSynced *[]cache.InformerSynced) {
	factory := k8sinformers.NewSharedInformerFactory(k.builtInClient, k.cacheResyncPeriod)
	ici := factory.Networking().V1().IngressClasses().Informer()
	k.addIngressClassHandlers(eventChan, ici)
	factory.Start(stop)
	*informersSynced = append(*informersSynced, ici.HasSynced)
}

func (k k8s) runProcessLevelGatewayClass(eventChan chan k8ssync.SyncDataEvent, stop chan struct{}, informersSynced *[]cache.InformerSynced) {
	factory := gatewaynetworking.NewSharedInformerFactory(k.gatewayClient, k.cacheResyncPeriod)
	gwclassInf := k.getGatewayClassesInformer(eventChan, factory)
	if gwclassInf != nil {
		factory.Start(stop)
		*informersSynced = append(*informersSynced, gwclassInf.HasSynced)
	}
}

func (k k8s) getSelectorNamespaceInformer(eventChan chan k8ssync.SyncDataEvent, factory k8sinformers.SharedInformerFactory) cache.SharedIndexInformer {
	informer := factory.Core().V1().Namespaces().Informer()
	errW := informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		go logger.Debug("Namespace selector informer error: %s", err)
	})
	logger.Error(errW)
	k.noteReg(informer.AddEventHandler(cache.ResourceEventHandlerDetailedFuncs{
		AddFunc: func(obj interface{}, isInInitialList bool) {
			item := namespaceStoreItem(obj, store.ADDED)
			if item == nil {
				return
			}
			ev := ToSyncDataEvent(item, item, "", "")
			ev.IsInInitialList = isInInitialList
			k.send(eventChan, ev)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			item := namespaceStoreItem(newObj, store.MODIFIED)
			if item == nil {
				return
			}
			k.send(eventChan, ToSyncDataEvent(item, item, "", ""))
		},
		DeleteFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				tombstone, tsOK := obj.(cache.DeletedFinalStateUnknown)
				if !tsOK {
					logger.Errorf("%s: Invalid data from k8s api, %s", k8ssync.NAMESPACE, obj)
					return
				}
				ns, ok = tombstone.Obj.(*corev1.Namespace)
				if !ok {
					logger.Errorf("%s: DeletedFinalStateUnknown contained non-Namespace object: %v", k8ssync.NAMESPACE, tombstone.Obj)
					return
				}
			}
			item := namespaceStoreItem(ns, store.DELETED)
			if item == nil {
				return
			}
			k.send(eventChan, ToSyncDataEvent(item, item, "", ""))
		},
	}))
	return informer
}

func namespaceStoreItem(obj interface{}, status store.Status) *store.Namespace {
	data, ok := obj.(*corev1.Namespace)
	if !ok {
		logger.Errorf("%s: Invalid data from k8s api, %s", k8ssync.NAMESPACE, obj)
		return nil
	}
	if status == store.ADDED && data.ObjectMeta.GetDeletionTimestamp() != nil {
		status = store.DELETED
	}
	return &store.Namespace{
		Name:                     data.GetName(),
		Endpoints:                make(map[string]map[string]*store.Endpoints),
		Services:                 make(map[string]*store.Service),
		Ingresses:                make(map[string]*store.Ingress),
		Secret:                   make(map[string]*store.Secret),
		HAProxyRuntime:           make(map[string]map[string]*store.RuntimeBackend),
		HAProxyRuntimeStandalone: make(map[string]map[string]map[string]*store.RuntimeBackend),
		CRs: &store.CustomResources{
			Global:    map[string]*models.Global{},
			Defaults:  map[string]*models.Defaults{},
			Backends:  map[string]*v3.BackendSpec{},
			TCPsPerCR: map[string]*store.TCPs{},
			Frontends: map[string]*v3.FrontendSpec{},
		},
		Gateways:        make(map[string]*store.Gateway),
		TCPRoutes:       make(map[string]*store.TCPRoute),
		ReferenceGrants: make(map[string]*store.ReferenceGrant),
		Labels:          utils.CopyMap(data.Labels),
		Status:          status,
	}
}
