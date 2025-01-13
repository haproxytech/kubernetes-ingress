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
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/crs/converters"
	informersv1 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v1/informers/externalversions"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type GlobalCRV1 struct{}

type DefaultsCRV1 struct{}

type BackendCRV1 struct{}

type TCPCRV1 struct{}

func NewGlobalCRV1() GlobalCRV1 {
	return GlobalCRV1{}
}

func NewDefaultsCRV1() DefaultsCRV1 {
	return DefaultsCRV1{}
}

func NewBackendCRV1() BackendCRV1 {
	return BackendCRV1{}
}

func NewTCPCRV1() TCPCRV1 {
	return TCPCRV1{}
}

func (c GlobalCRV1) GetKind() string {
	return "Global"
}

func (c GlobalCRV1) GetInformerV1(eventChan chan k8ssync.SyncDataEvent, factory informersv1.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Ingress().V1().Globals().Informer()

	sendToChannel := func(eventChan chan k8ssync.SyncDataEvent, object interface{}, status store.Status) {
		dataV1, ok := object.(*v1.Global)
		if !ok {
			logger.Warning(CRSGroupVersionV1 + ": type mismatch with Global kind")
			return
			// will contain the converted CR (converted to the latest API group
			// "ingress.haproxy.org/v3").
		}
		logger.Warningf("Global CR defined in API %s is DEPRECATED, please upgrade", CRSGroupVersionV1)
		data := &v3.Global{}
		data.TypeMeta = dataV1.TypeMeta
		data.ObjectMeta = dataV1.ObjectMeta
		data.Spec = converters.DeepConvertGlobalSpecV1toV3(dataV1.Spec)

		logger.Debugf("%s %s: %s", data.GetNamespace(), status, data.GetName())
		if status == store.DELETED {
			eventChan <- k8ssync.SyncDataEvent{
				SyncType:  k8ssync.SyncType(c.GetKind()),
				Namespace: data.GetNamespace(), Name: data.GetName(), Data: nil,
			}
			return
		}
		eventChan <- k8ssync.SyncDataEvent{
			SyncType:  k8ssync.SyncType(c.GetKind()),
			Namespace: data.GetNamespace(), Name: data.GetName(), Data: data,
		}
	}

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

func (c DefaultsCRV1) GetKind() string {
	return "Defaults"
}

func (c DefaultsCRV1) GetInformerV1(eventChan chan k8ssync.SyncDataEvent, factory informersv1.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Ingress().V1().Defaults().Informer()

	sendToChannel := func(eventChan chan k8ssync.SyncDataEvent, object interface{}, status store.Status) {
		dataV1, ok := object.(*v1.Defaults)
		if !ok {
			logger.Warning(CRSGroupVersionV1 + ": type mismatch with Defaults kind")
			return
		}
		logger.Warningf("Defaults CR defined in API %s is DEPRECATED, please upgrade", CRSGroupVersionV1)
		data := &v3.Defaults{}
		data.TypeMeta = dataV1.TypeMeta
		data.ObjectMeta = dataV1.ObjectMeta
		data.Spec = converters.DeepConvertDefaultsSpecV1toV3(dataV1.Spec)
		logger.Debugf("%s %s: %s", data.GetNamespace(), status, data.GetName())
		if status == store.DELETED {
			eventChan <- k8ssync.SyncDataEvent{
				SyncType:  k8ssync.SyncType(c.GetKind()),
				Namespace: data.GetNamespace(), Name: data.GetName(), Data: nil,
			}
			return
		}
		eventChan <- k8ssync.SyncDataEvent{
			SyncType:  k8ssync.SyncType(c.GetKind()),
			Namespace: data.GetNamespace(), Name: data.GetName(), Data: data,
		}
	}

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

func (c BackendCRV1) GetKind() string {
	return "Backend"
}

func (c BackendCRV1) GetInformerV1(eventChan chan k8ssync.SyncDataEvent, factory informersv1.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Ingress().V1().Backends().Informer()

	sendToChannel := func(eventChan chan k8ssync.SyncDataEvent, object interface{}, status store.Status) {
		dataV1, ok := object.(*v1.Backend)
		if !ok {
			logger.Warning(CRSGroupVersionV1 + ": type mismatch with Backend kind")
			return
		}
		logger.Warningf("Backend CR defined in API %s is DEPRECATED, please upgrade", CRSGroupVersionV1)
		data := &v3.Backend{}
		data.TypeMeta = dataV1.TypeMeta
		data.ObjectMeta = dataV1.ObjectMeta
		data.Spec = converters.DeepConvertBackendSpecV1toV3(dataV1.Spec)

		logger.Debugf("%s %s: %s", data.GetNamespace(), status, data.GetName())
		if status == store.DELETED {
			eventChan <- k8ssync.SyncDataEvent{
				SyncType:  k8ssync.SyncType(c.GetKind()),
				Namespace: data.GetNamespace(), Name: data.GetName(), Data: nil,
			}
			return
		}
		eventChan <- k8ssync.SyncDataEvent{
			SyncType:  k8ssync.SyncType(c.GetKind()),
			Namespace: data.GetNamespace(), Name: data.GetName(), Data: data,
		}
	}

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

func (c TCPCRV1) GetKind() string {
	return "TCP"
}

func (c TCPCRV1) GetInformerV1(eventChan chan k8ssync.SyncDataEvent, factory informersv1.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Ingress().V1().TCPs().Informer()

	sendToChannel := func(eventChan chan k8ssync.SyncDataEvent, object interface{}, status store.Status) {
		dataV1, ok := object.(*v1.TCP)
		if !ok {
			logger.Warning(CRSGroupVersionV1 + ": type mismatch with Global kind")
			return
		}
		logger.Warningf("Global CR defined in API %s is DEPRECATED, please upgrade", CRSGroupVersionV1)
		data := &v3.TCP{}
		data.TypeMeta = dataV1.TypeMeta
		data.ObjectMeta = dataV1.ObjectMeta
		data.Spec = converters.DeepConvertTCPSpecV1toV3(dataV1.Spec)

		logger.Debugf("%s %s: %s", data.GetNamespace(), status, data.GetName())
		storeTCP := convertToStoreTCP(data, status)
		if storeTCP == nil {
			logger.Warning("convertToStoreTCP failed for %v", data)
			return
		}

		eventChan <- k8ssync.SyncDataEvent{
			SyncType:  k8ssync.SyncType(c.GetKind()),
			Namespace: data.GetNamespace(),
			Name:      data.GetName(),
			Data:      storeTCP,
		}
	}

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
