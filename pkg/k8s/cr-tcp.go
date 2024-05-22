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
	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
	informers "github.com/haproxytech/kubernetes-ingress/crs/generated/informers/externalversions"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"k8s.io/client-go/tools/cache"
)

type TCPCR struct{}

func NewTCPCR() TCPCR {
	return TCPCR{}
}

func (c TCPCR) GetInformer(eventChan chan k8ssync.SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Ingress().V1().TCPs().Informer()

	sendToChannel := func(eventChan chan k8ssync.SyncDataEvent, newObject interface{}, status store.Status) {
		storeTCP := convertToStoreTCP(newObject, status)
		if storeTCP == nil {
			return
		}

		logger.Debugf("%s %s: %s", storeTCP.Namespace, status, storeTCP.Name)
		eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.SyncType(c.GetKind()), Namespace: storeTCP.Namespace, Name: storeTCP.Name, Data: storeTCP}
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

func (c TCPCR) GetKind() string {
	return "TCP"
}

func convertToStoreTCP(k8sData interface{}, status store.Status) *store.TCPs {
	data, ok := k8sData.(*v1.TCP)
	if !ok {
		logger.Warning(CRSGroupVersionV1 + ": type mismatch with TCP CR kind")
		return nil
	}
	storeTCP := store.TCPs{
		Status:    status,
		Namespace: data.GetNamespace(),
		Name:      data.GetName(),
		Items:     make([]*store.TCPResource, 0),
	}
	for _, tcp := range data.Spec.TCPs {
		storeTCP.Items = append(storeTCP.Items, &store.TCPResource{
			CreationTimestamp: data.CreationTimestamp.Time,
			TCPModel:          tcp,
			Namespace:         data.Namespace,
			ParentName:        data.Name,
		})
	}

	return &storeTCP
}
