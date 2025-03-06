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

	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	informers "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v3/informers/externalversions"
	k8stransform "github.com/haproxytech/kubernetes-ingress/pkg/k8s/transform"

	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type BackendCR struct{}

func NewBackendCRV3() BackendCR {
	return BackendCR{}
}

func (c BackendCR) GetKind() string {
	return "Backend"
}

func (c BackendCR) GetInformerV3(eventChan chan k8ssync.SyncDataEvent, factory informers.SharedInformerFactory, osArgs utils.OSArgs) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Ingress().V3().Backends().Informer()

	sendToChannel := func(eventChan chan k8ssync.SyncDataEvent, object interface{}, status store.Status) {
		data, ok := object.(*v3.Backend)
		if !ok {
			logger.Warning(CRSGroupVersionV3 + ": type mismatch with Backend kind")
			return
		}
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
	// Use TransformFunc to modify/filter objects before passing them to handlers
	err = informer.SetTransform(k8stransform.TransformBackend)
	logger.Error(err)
	return informer
}
