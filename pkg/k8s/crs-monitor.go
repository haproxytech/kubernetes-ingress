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
	"strings"
	"time"

	"k8s.io/client-go/tools/cache"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	// apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
)

func (k k8s) runCRDefinitionsInformer(eventChan chan string, stop chan struct{}) { //nolint:ireturn
	// Create a new informer factory with the clientset.

	factory := apiextensionsinformers.NewSharedInformerFactoryWithOptions(k.apiExtensionsClient, k.cacheResyncPeriod)
	informer := factory.Apiextensions().V1().CustomResourceDefinitions().Informer()
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			crd := obj.(*apiextensionsv1.CustomResourceDefinition)
			if crd.Spec.Group != "core.haproxy.org" {
				return
			}
			if !(crd.Spec.Names.Kind == "Global" || crd.Spec.Names.Kind == "Defaults" || crd.Spec.Names.Kind == "Backend") {
				return
			}
			for _, version := range crd.Spec.Versions {
				if version.Name == "v1alpha2" {
					time.Sleep(time.Second * 5) // a little delay is needed to let CRD API be created
					eventChan <- crd.Spec.Names.Kind
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

func (k k8s) RunCRSCreationMonitoring(eventChan chan SyncDataEvent, stop chan struct{}) {
	count := 0
	for key := range k.crs {
		if strings.Contains(key, "core.haproxy.org/v1alpha2") {
			count++
		}
	}
	if count > 2 {
		// all crds are already in list
		return
	}

	eventCRS := make(chan string)
	k.runCRDefinitionsInformer(eventCRS, stop)
	go func(chan string) {
		for {
			select {
			case crdName := <-eventCRS:
				if _, ok := k.crs["core.haproxy.org/v1alpha2 - "+crdName]; ok {
					// we have already created watchers for this CRD
					continue
				}
				informersSyncedEvent := &[]cache.InformerSynced{}
				for _, namespace := range k.whiteListedNS {
					crs := map[string]CR{}
					switch crdName {
					case "Backend":
						crs[crdName] = NewBackendCR()
					case "Defaults":
						crs[crdName] = NewDefaultsCR()
					case "Global":
						crs[crdName] = NewGlobalCR()
					}
					logger.Info("Custom resource definition created, adding CR watcher for " + crs[crdName].GetKind())
					k.runCRInformers(eventChan, stop, namespace, informersSyncedEvent, crs)
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
