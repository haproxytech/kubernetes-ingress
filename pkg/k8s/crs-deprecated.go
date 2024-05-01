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

	corev1alpha1 "github.com/haproxytech/kubernetes-ingress/crs/api/core/v1alpha1"
	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
	"github.com/haproxytech/kubernetes-ingress/crs/converters"
	informers "github.com/haproxytech/kubernetes-ingress/crs/generated/informers/externalversions"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type GlobalCRV1Alpha1 struct{}

type DefaultsCRV1Alpha1 struct{}

type BackendCRV1Alpha1 struct{}

func NewGlobalCRV1Alpha1() GlobalCRV1Alpha1 {
	return GlobalCRV1Alpha1{}
}

func NewDefaultsCRV1Alpha1() DefaultsCRV1Alpha1 {
	return DefaultsCRV1Alpha1{}
}

func NewBackendCRV1Alpha1() BackendCRV1Alpha1 {
	return BackendCRV1Alpha1{}
}

func (c GlobalCRV1Alpha1) GetKind() string {
	return "Global"
}

func (c GlobalCRV1Alpha1) GetInformer(eventChan chan k8ssync.SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Core().V1alpha1().Globals().Informer()

	sendToChannel := func(eventChan chan k8ssync.SyncDataEvent, object interface{}, status store.Status) {
		dataV1Alpha1, ok := object.(*corev1alpha1.Global)
		if !ok {
			logger.Warning(CRSGroupVersionV1alpha1 + ": type mismatch with Global kind")
			return
		}
		logger.Warningf("Global CR defined in API %s is DEPRECATED, please upgrade", CRSGroupVersionV1alpha1)
		data := &v1.Global{}
		data.TypeMeta = dataV1Alpha1.TypeMeta
		data.ObjectMeta = dataV1Alpha1.ObjectMeta
		spec := converters.DeepConvertGlobalSpecA1toA2(dataV1Alpha1.Spec)
		data.Spec = converters.DeepConvertGlobalSpecA2toV1(spec)

		logger.Debugf("%s %s: %s", data.GetNamespace(), status, data.GetName())
		if status == store.DELETED {
			eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.SyncType(c.GetKind()), Namespace: data.GetNamespace(), Name: data.GetName(), Data: nil}
			return
		}
		eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.SyncType(c.GetKind()), Namespace: data.GetNamespace(), Name: data.GetName(), Data: data}
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

func (c DefaultsCRV1Alpha1) GetKind() string {
	return "Defaults"
}

func (c DefaultsCRV1Alpha1) GetInformer(eventChan chan k8ssync.SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Core().V1alpha1().Defaults().Informer()

	sendToChannel := func(eventChan chan k8ssync.SyncDataEvent, object interface{}, status store.Status) {
		dataV1Alpha1, ok := object.(*corev1alpha1.Defaults)
		if !ok {
			logger.Warning(CRSGroupVersionV1alpha1 + ": type mismatch with Defaults kind")
			return
		}
		logger.Warningf("Defaults CR defined in API %s is DEPRECATED, please upgrade", CRSGroupVersionV1alpha1)
		data := &v1.Defaults{}
		data.TypeMeta = dataV1Alpha1.TypeMeta
		data.ObjectMeta = dataV1Alpha1.ObjectMeta
		spec := converters.DeepConvertDefaultsSpecA1toA2(dataV1Alpha1.Spec)
		data.Spec = converters.DeepConvertDefaultsSpecA2toV1(spec)
		logger.Debugf("%s %s: %s", data.GetNamespace(), status, data.GetName())
		if status == store.DELETED {
			eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.SyncType(c.GetKind()), Namespace: data.GetNamespace(), Name: data.GetName(), Data: nil}
			return
		}
		eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.SyncType(c.GetKind()), Namespace: data.GetNamespace(), Name: data.GetName(), Data: data}
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

func (c BackendCRV1Alpha1) GetKind() string {
	return "Backend"
}

func (c BackendCRV1Alpha1) GetInformer(eventChan chan k8ssync.SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Core().V1alpha1().Backends().Informer()

	sendToChannel := func(eventChan chan k8ssync.SyncDataEvent, object interface{}, status store.Status) {
		dataV1Alpha1, ok := object.(*corev1alpha1.Backend)
		if !ok {
			logger.Warning(CRSGroupVersionV1alpha1 + ": type mismatch with Backend kind")
			return
		}
		logger.Warningf("Backend CR defined in API %s is DEPRECATED, please upgrade", CRSGroupVersionV1alpha1)
		data := &v1.Backend{}
		data.TypeMeta = dataV1Alpha1.TypeMeta
		data.ObjectMeta = dataV1Alpha1.ObjectMeta
		spec := converters.DeepConvertBackendSpecA1toA2(dataV1Alpha1.Spec)
		data.Spec = converters.DeepConvertBackendSpecA2toV1(spec)

		logger.Debugf("%s %s: %s", data.GetNamespace(), status, data.GetName())
		if status == store.DELETED {
			eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.SyncType(c.GetKind()), Namespace: data.GetNamespace(), Name: data.GetName(), Data: nil}
			return
		}
		eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.SyncType(c.GetKind()), Namespace: data.GetNamespace(), Name: data.GetName(), Data: data}
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
