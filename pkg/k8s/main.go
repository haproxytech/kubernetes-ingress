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
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	cr "github.com/haproxytech/kubernetes-ingress/crs/generated/clientset/versioned"
	informers "github.com/haproxytech/kubernetes-ingress/crs/generated/informers/externalversions"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetK8sAPILogger()

// TRACE_API outputs all k8s events received from k8s API
//nolint golint
const (
	TRACE_API        = false
	CoreGroupVersion = "core.haproxy.org/v1alpha1"
)

var ErrIgnored = errors.New("ignored resource")

// A Custom Resource interface
// Any CR should be able to provide its kind, its kubernetes Informer
// and a method to process the update of a CR
type CR interface {
	GetKind() string
	GetInformer(chan SyncDataEvent, informers.SharedInformerFactory) cache.SharedIndexInformer
}

// K8s is structure with all data required to synchronize with k8s
type K8s struct {
	builtInClient          *kubernetes.Clientset
	crClient               *cr.Clientset
	crs                    map[string]CR
	cacheResync            time.Duration
	events                 chan SyncDataEvent
	disableSvcExternalName bool // CVE-2021-25740
}

func New(restconfig *rest.Config, osArgs utils.OSArgs, events chan SyncDataEvent) K8s {
	if !TRACE_API {
		logger.SetLevel(utils.Info)
	}
	k := K8s{
		builtInClient:          kubernetes.NewForConfigOrDie(restconfig),
		crClient:               cr.NewForConfigOrDie(restconfig),
		crs:                    map[string]CR{},
		cacheResync:            osArgs.CacheResyncPeriod,
		disableSvcExternalName: osArgs.DisableServiceExternalName,
		events:                 events,
	}
	k.RegisterCoreCR(NewGlobalCR())
	k.RegisterCoreCR(NewDefaultsCR())
	k.RegisterCoreCR(NewBackendCR())
	return k
}

func (k K8s) GetClient() *kubernetes.Clientset {
	return k.builtInClient
}

func (k K8s) RegisterCoreCR(cr CR) {
	resources, err := k.crClient.DiscoveryClient.ServerResourcesForGroupVersion(CoreGroupVersion)
	if err != nil {
		return
	}
	logger.Debugf("Custom API core.haproxy.org available")
	kindName := cr.GetKind()
	for _, resource := range resources.APIResources {
		if resource.Kind == kindName {
			k.crs[kindName] = cr
			logger.Infof("%s CR defined in API core.haproxy.org", kindName)
			break
		}
	}
}

// RunCRInformers runs Custom Resource Informers and return an array of corresponding cache.InformerSynced
func (k K8s) RunCRInformers(namespace string, stop chan struct{}) []cache.InformerSynced {
	informerSynced := make([]cache.InformerSynced, 0, len(k.crs))
	informerFactory := informers.NewSharedInformerFactoryWithOptions(k.crClient, k.cacheResync, informers.WithNamespace(namespace))
	for _, controller := range k.crs {
		informer := controller.GetInformer(k.events, informerFactory)
		go informer.Run(stop)
		informerSynced = append(informerSynced, informer.HasSynced)
	}
	return informerSynced
}
