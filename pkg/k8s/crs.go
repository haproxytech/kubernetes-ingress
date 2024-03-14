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
	"k8s.io/client-go/tools/cache"

	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
	informers "github.com/haproxytech/kubernetes-ingress/crs/generated/informers/externalversions"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type GlobalCR struct{}

type DefaultsCR struct{}

type BackendCR struct{}

func NewGlobalCR() GlobalCR {
	return GlobalCR{}
}

func NewDefaultsCR() DefaultsCR {
	return DefaultsCR{}
}

func NewBackendCR() BackendCR {
	return BackendCR{}
}

func (c GlobalCR) GetKind() string {
	return "Global"
}

func (c GlobalCR) GetInformer(eventChan chan SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Ingress().V1().Globals().Informer()

	sendToChannel := func(eventChan chan SyncDataEvent, object interface{}, status store.Status) {
		data, ok := object.(*v1.Global)
		if !ok {
			logger.Warning(CRSGroupVersionV1 + ": type mismatch with Global kind")
			return
		}
		logger.Debugf("%s %s: %s", data.GetNamespace(), status, data.GetName())
		if status == store.DELETED {
			eventChan <- SyncDataEvent{SyncType: SyncType(c.GetKind()), Namespace: data.GetNamespace(), Name: data.GetName(), Data: nil}
			return
		}
		eventChan <- SyncDataEvent{SyncType: SyncType(c.GetKind()), Namespace: data.GetNamespace(), Name: data.GetName(), Data: data}
	}

	errW := informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		go logger.Debug("Global CR informer error: %s", err)
	})
	logger.Error(errW)
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			sendToChannel(eventChan, obj, store.ADDED)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			sendToChannel(eventChan, newObj, store.MODIFIED)
		},
		DeleteFunc: func(obj interface{}) {
			sendToChannel(eventChan, obj, store.DELETED)
		},
	})
	logger.Error(err)
	return informer
}

func (c DefaultsCR) GetKind() string {
	return "Defaults"
}

func (c DefaultsCR) GetInformer(eventChan chan SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Ingress().V1().Defaults().Informer()

	sendToChannel := func(eventChan chan SyncDataEvent, object interface{}, status store.Status) {
		data, ok := object.(*v1.Defaults)
		if !ok {
			logger.Warning(CRSGroupVersionV1 + ": type mismatch with Defaults kind")
			return
		}
		logger.Debugf("%s %s: %s", data.GetNamespace(), status, data.GetName())
		if status == store.DELETED {
			eventChan <- SyncDataEvent{SyncType: SyncType(c.GetKind()), Namespace: data.GetNamespace(), Name: data.GetName(), Data: nil}
			return
		}
		eventChan <- SyncDataEvent{SyncType: SyncType(c.GetKind()), Namespace: data.GetNamespace(), Name: data.GetName(), Data: data}
	}

	errW := informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		go logger.Debug("Defaults CR informer error: %s", err)
	})
	logger.Error(errW)
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			sendToChannel(eventChan, obj, store.ADDED)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			sendToChannel(eventChan, newObj, store.MODIFIED)
		},
		DeleteFunc: func(obj interface{}) {
			sendToChannel(eventChan, obj, store.DELETED)
		},
	})
	logger.Error(err)
	return informer
}

func (c BackendCR) GetKind() string {
	return "Backend"
}

func (c BackendCR) GetInformer(eventChan chan SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Ingress().V1().Backends().Informer()

	sendToChannel := func(eventChan chan SyncDataEvent, object interface{}, status store.Status) {
		data, ok := object.(*v1.Backend)
		if !ok {
			logger.Warning(CRSGroupVersionV1 + ": type mismatch with Backend kind")
			return
		}
		logger.Debugf("%s %s: %s", data.GetNamespace(), status, data.GetName())
		if status == store.DELETED {
			eventChan <- SyncDataEvent{SyncType: SyncType(c.GetKind()), Namespace: data.GetNamespace(), Name: data.GetName(), Data: nil}
			return
		}
		eventChan <- SyncDataEvent{SyncType: SyncType(c.GetKind()), Namespace: data.GetNamespace(), Name: data.GetName(), Data: data}
	}

	errW := informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		go logger.Debug("Backend CR informer error: %s", err)
	})
	logger.Error(errW)
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			sendToChannel(eventChan, obj, store.ADDED)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			sendToChannel(eventChan, newObj, store.MODIFIED)
		},
		DeleteFunc: func(obj interface{}) {
			sendToChannel(eventChan, obj, store.DELETED)
		},
	})
	logger.Error(err)
	return informer
}
