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
	"errors"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8sinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	crinformersv1 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v1/informers/externalversions"
	crinformersv3 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v3/informers/externalversions"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	k8stransform "github.com/haproxytech/kubernetes-ingress/pkg/k8s/transform"
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
		return nil, errors.New("session manager is nil")
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
	ii, _ := sk.getIngressInformers(proxy, core, k.osArgs)
	if ii == nil {
		close(proxy)
		stampWg.Wait()
		return nil, errors.New("ingress resource not supported")
	}
	epsi := sk.getEndpointSliceInformer(proxy, core)
	if epsi == nil || !k.hasEndpointSliceMirroring {
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

	logger.Infof("namespace-label-selector %q: watching all Namespace objects, matching locally", selector)
	nsFactory := k8sinformers.NewSharedInformerFactoryWithOptions(k.builtInClient, k.cacheResyncPeriod)
	var nsRegs []cache.ResourceEventHandlerRegistration
	sk := k
	sk.handlerRegs = &nsRegs
	nsInformer := sk.getSelectorNamespaceInformer(eventChan, nsFactory)
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
	k.warnUnwatchedConfigNamespaces(osArgs)
	if !k.sessions.WaitAllReady(stop, osArgs.NamespaceSelectorReadyTimeout) {
		return
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

// warnUnwatchedConfigNamespaces logs when a namespace holding an object the
// controller reads through the store is not matched by the label selector,
// because the object is then silently ignored. This is hard to diagnose from
// the outside, e.g. an unmatched host loses the default backend. ConfigMap
// namespaces are excluded: they are watched at process level.
func (k k8s) warnUnwatchedConfigNamespaces(osArgs utils.OSArgs) {
	if k.sessions == nil {
		return
	}
	publishNamespace := ""
	if k.publishSvc != nil {
		publishNamespace = k.publishSvc.Namespace
	}
	refs := []struct {
		what      string
		namespace string
	}{
		{"--custom-validation-rules", osArgs.CustomValidationRules.Namespace},
		{"--default-backend-service", osArgs.DefaultBackendService.Namespace},
		{"--default-ssl-certificate", osArgs.DefaultCertificate.Namespace},
		{"--publish-service", publishNamespace},
	}
	// With --default-backend-service unset the controller namespace hosts the
	// built-in local default backend; without a session it is silently missing.
	if osArgs.DefaultBackendService.String() == "" && k.podNamespace != "" {
		refs = append(refs, struct {
			what      string
			namespace string
		}{"the built-in local default backend", k.podNamespace})
	}

	warned := map[string]struct{}{}
	for _, ref := range refs {
		if ref.namespace == "" {
			continue
		}
		if _, ok := warned[ref.namespace]; ok {
			continue
		}
		k.sessions.mu.RLock()
		_, watched := k.sessions.sessions[ref.namespace]
		k.sessions.mu.RUnlock()
		if !watched {
			warned[ref.namespace] = struct{}{}
			logger.Warningf("namespace-label-selector: %s needs namespace %q, which does not match the selector; its objects will not be used", ref.what, ref.namespace)
		}
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
	k.noteReg(informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			item := namespaceStoreItem(obj, store.ADDED)
			if item == nil {
				return
			}
			k.send(eventChan, ToSyncDataEvent(item, item, "", ""))
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
	err := informer.SetTransform(k8stransform.TransformNamespace)
	logger.Error(err)
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
		Name:   data.GetName(),
		Labels: utils.CopyMap(data.Labels),
		Status: status,
	}
}
