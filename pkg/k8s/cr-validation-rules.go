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
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	informersv3 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v3/informers/externalversions"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/validators"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/client-go/tools/cache"
)

type ValidationCR struct{}

func NewValidationCRV3() ValidationCR {
	return ValidationCR{}
}

func (c ValidationCR) GetInformerV3(eventChan chan k8ssync.SyncDataEvent, factory informersv3.SharedInformerFactory, osArgs utils.OSArgs) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Ingress().V3().ValidationRules().Informer()

	sendToChannel := func(eventChan chan k8ssync.SyncDataEvent, newObject interface{}, status store.Status) {
		data, ok := newObject.(*v3.ValidationRules)
		if !ok {
			logger.Warning(CRSGroupVersionV3 + ": type mismatch with ValidationRules kind")
			return
		}
		ingressClass := data.Annotations["ingress.class"]

		logger.Debugf("%s %s: %s", data.Namespace, status, data.Name)
		if ingressClass != "" && ingressClass != osArgs.IngressClass {
			// Due to ingressclass.kubernetes.io/is-default-class annotation in ingressclass
			// we need to keep also empty ingressclasses in ingress
			return
		}

		if data.ObjectMeta.Namespace != osArgs.CustomValidationRules.Namespace {
			return
		}
		if data.ObjectMeta.Name != osArgs.CustomValidationRules.Name {
			return
		}

		validator, err := validators.Get()
		if err != nil {
			logger.Error("Failed to get validator: %s", err)
			return
		}
		err = validator.Set(data.Spec.Prefix, data.Spec.Config)
		if err != nil {
			logger.Error("Failed to set validation rules: %s", err)
			return
		}
		logger.Infof("ValidationRules %s/%s accepted and set [%s]", data.Namespace, data.Name, data.Spec.Prefix)
		// eventChan <- k8ssync.SyncDataEvent{
		// 	SyncType: k8ssync.SyncType(c.GetKind()), Namespace: data.Namespace, Name: data.Name, Data: data,
		// }
		eventChan <- k8ssync.SyncDataEvent{
			SyncType: k8ssync.CUSTOM_RESOURCE, Namespace: data.Namespace, Name: data.Name, Data: data,
		}
	}

	errW := informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		go logger.Debug("ValidationRules CR informer error: %s", err)
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

func (c ValidationCR) GetKind() string {
	return "ValidationRules"
}
