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
				k.rememberLateCR(groupKind, crsV1, crsV3)
				if k.sessions != nil {
					sessions := k.sessions.snapshot()
					for _, sess := range sessions {
						var synced []cache.InformerSynced
						sink := sessionResourceChan(sess, eventChan)
						k.runCRInformers(sink, sess.stopCh, sess.namespace, &synced, crsV1, crsV3, osArgs, false, sess.crV1, sess.crV3)
						if sess.crV1 != nil {
							sess.crV1.Start(sess.stopCh)
						}
						if sess.crV3 != nil {
							sess.crV3.Start(sess.stopCh)
						}
						if len(synced) > 0 {
							cache.WaitForCacheSync(sess.stopCh, synced...)
						}
					}
					// WaitForCacheSync only means handlers sent on the proxy.
					// Drain each proxy so those List events are in the store
					// before the COMMAND rebuild (the drain COMMAND itself).
					for _, sess := range sessions {
						if drainSessionEvents(sess, eventChan, stop) {
							return
						}
					}
					continue
				}
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
