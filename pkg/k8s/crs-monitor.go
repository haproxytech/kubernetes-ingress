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
	"time"

	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/client-go/tools/cache"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
)

type GroupKind struct {
	Group string
	Kind  string
}

func (k k8s) runCRDefinitionsInformer(eventChan chan GroupKind, stop chan struct{}) { //nolint:ireturn
	// Create a new informer factory with the clientset.

	factory := apiextensionsinformers.NewSharedInformerFactoryWithOptions(k.apiExtensionsClient, k.cacheResyncPeriod)
	informer := factory.Apiextensions().V1().CustomResourceDefinitions().Informer()
	errW := informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		go logger.Debug("CRD Definitions informer error: %s", err)
	})
	logger.Error(errW)
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			//revive:disable-next-line:unchecked-type-assertion
			crd := obj.(*apiextensionsv1.CustomResourceDefinition)
			if !(crd.Spec.Group == "ingress.v1.haproxy.org" || crd.Spec.Group == "ingress.v3.haproxy.org") {
				return
			}
			if !(crd.Spec.Names.Kind == "Global" ||
				crd.Spec.Names.Kind == "Defaults" ||
				crd.Spec.Names.Kind == "Backend" ||
				crd.Spec.Names.Kind == "TCP" ||
				crd.Spec.Names.Kind == "Frontend" ||
				crd.Spec.Names.Kind == "ValidationRules") {
				return
			}
			for _, version := range crd.Spec.Versions {
				if (version.Name == "v1" && crd.Spec.Group == "ingress.v1.haproxy.org") ||
					(version.Name == "v3" && crd.Spec.Group == "ingress.v3.haproxy.org") {
					if _, ok := k.crsRegisteredOnStart[crd.Spec.Group+" - "+crd.Spec.Names.Kind]; !ok {
						time.Sleep(time.Second * 5) // a little delay is needed to let CRD API be created
					}
					eventChan <- GroupKind{
						Group: crd.Spec.Group,
						Kind:  crd.Spec.Names.Kind,
					}
					return
				}
			}
		},
	})

	go informer.Run(stop)

	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		logger.Error("Caches are not populated due to an underlying error, cannot monitor CRS creation")
	}

	logger.Error(err)
}

//revive:disable-next-line:cognitive-complexity
func (k k8s) RunCRSCreationMonitoring(eventChan chan k8ssync.SyncDataEvent, stop chan struct{}, osArgs utils.OSArgs) {
	eventCRS := make(chan GroupKind)
	k.runCRDefinitionsInformer(eventCRS, stop)
	go func(chan GroupKind) {
		for {
			select {
			case groupKind := <-eventCRS:
				if k.crAlreadyWatched(groupKind) {
					continue
				}
				informersSyncedEvent := &[]cache.InformerSynced{}
				crsV1, crsV3 := lateCRMaps(groupKind, osArgs)
				if k.sessions != nil {
					k.registerSessionCR(groupKind, crsV1, crsV3, eventChan, stop, osArgs)
					continue
				}
				k.rememberLateCR(groupKind, crsV1, crsV3)
				for _, namespace := range k.whiteListedNS {
					k.runCRInformers(eventChan, stop, namespace, informersSyncedEvent, crsV1, crsV3, osArgs, true, nil, nil)
				}

				if !cache.WaitForCacheSync(stop, *informersSyncedEvent...) {
					logger.Error("Caches are not populated due to an underlying error, cannot monitor new CRDs")
				}
			case <-stop:
				return
			}
		}
	}(eventCRS)
}

// registerSessionCR serializes publication of the CR set with session.run and
// Stop. Placeholders consume the updated set when their construction completes.
func (k k8s) registerSessionCR(groupKind GroupKind, crsV1 map[string]CRV1, crsV3 map[string]CRV3,
	eventChan chan k8ssync.SyncDataEvent, stop <-chan struct{}, osArgs utils.OSArgs,
) {
	k.sessions.mu.Lock()
	defer k.sessions.mu.Unlock()
	if k.sessions.closed || k.crAlreadyWatched(groupKind) {
		return
	}
	k.rememberLateCR(groupKind, crsV1, crsV3)
	for _, sess := range k.sessions.sessions {
		if !sess.constructed {
			continue
		}
		var synced []cache.InformerSynced
		var regs []cache.ResourceEventHandlerRegistration
		sk := k
		sk.handlerRegs = &regs
		sk.runCRInformers(sessionResourceChan(sess, eventChan), sess.stopCh, sess.namespace,
			&synced, crsV1, crsV3, osArgs, false, sess.crV1, sess.crV3)
		// Handler sync, rather than just informer sync, guarantees initial
		// callbacks have sent their events before the FIFO drain.
		for _, reg := range regs {
			if reg != nil {
				synced = append(synced, reg.HasSynced)
			}
		}
		sess.crV1.Start(sess.stopCh)
		sess.crV3.Start(sess.stopCh)
		go syncSessionCR(sess, synced, eventChan, stop)
	}
}

// Each CRD/session pair waits independently: a failed LIST must not prevent
// another namespace or a subsequently discovered CRD from starting.
func syncSessionCR(sess *nsSession, synced []cache.InformerSynced, eventChan chan k8ssync.SyncDataEvent, stop <-chan struct{}) {
	waitStop := make(chan struct{})
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-stop:
		case <-sess.stopCh:
		case <-done:
		}
		close(waitStop)
	}()
	if cache.WaitForCacheSync(waitStop, synced...) {
		drainSessionEvents(sess, eventChan, stop)
	}
}

func crMapKey(group, kind string) string {
	return group + " - " + kind
}

// crAlreadyWatched reports whether this CRD is already registered.
func (k k8s) crAlreadyWatched(groupKind GroupKind) bool {
	if k.crsMu != nil {
		k.crsMu.RLock()
		defer k.crsMu.RUnlock()
	}
	if groupKind.Group == "ingress.v1.haproxy.org" {
		_, ok := k.crsV1[crMapKey(groupKind.Group, groupKind.Kind)]
		return ok
	}
	if groupKind.Group == "ingress.v3.haproxy.org" {
		_, ok := k.crsV3[crMapKey(groupKind.Group, groupKind.Kind)]
		return ok
	}
	return false
}

func (k k8s) rememberLateCR(groupKind GroupKind, crsV1 map[string]CRV1, crsV3 map[string]CRV3) {
	if k.crsMu != nil {
		k.crsMu.Lock()
		defer k.crsMu.Unlock()
	}
	for kind, cr := range crsV1 {
		k.crsV1[crMapKey(groupKind.Group, kind)] = cr
	}
	for kind, cr := range crsV3 {
		k.crsV3[crMapKey(groupKind.Group, kind)] = cr
	}
}

func lateCRMaps(groupKind GroupKind, osArgs utils.OSArgs) (map[string]CRV1, map[string]CRV3) {
	crsV1 := map[string]CRV1{}
	crsV3 := map[string]CRV3{}
	switch groupKind.Group {
	case "ingress.v1.haproxy.org":
		switch groupKind.Kind {
		case "Backend":
			crsV1[groupKind.Kind] = NewBackendCRV1()
		case "Defaults":
			crsV1[groupKind.Kind] = NewDefaultsCRV1()
		case "Global":
			crsV1[groupKind.Kind] = NewGlobalCRV1()
		case "TCP":
			crsV1[groupKind.Kind] = NewTCPCRV1()
		}
		if cr, ok := crsV1[groupKind.Kind]; ok {
			logger.Info("Custom resource definition created, adding CR watcher for " + cr.GetKind())
		}
	case "ingress.v3.haproxy.org":
		switch groupKind.Kind {
		case "Backend":
			crsV3[groupKind.Kind] = NewBackendCRV3()
		case "Defaults":
			crsV3[groupKind.Kind] = NewDefaultsCRV3()
		case "Global":
			crsV3[groupKind.Kind] = NewGlobalCRV3()
		case "TCP":
			crsV3[groupKind.Kind] = NewTCPCRV3()
		case "ValidationRules":
			if osArgs.CustomValidationRules.Name != "" {
				crsV3[groupKind.Kind] = NewValidationCRV3()
			}
		case "Frontend":
			crsV3[groupKind.Kind] = NewFrontendCRV3()
		}
		if cr, ok := crsV3[groupKind.Kind]; ok {
			logger.Info("Custom resource definition created, adding CR watcher for " + cr.GetKind() + " " + groupKind.Group)
		}
	}
	return crsV1, crsV3
}
