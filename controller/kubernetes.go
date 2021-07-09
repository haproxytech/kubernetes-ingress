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

package controller

import (
	"errors"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	ingstatus "github.com/haproxytech/kubernetes-ingress/controller/status"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// TRACE_API outputs all k8s events received from k8s API
//nolint golint
const (
	TRACE_API = false
)

var ErrIgnored = errors.New("ignored resource")

// K8s is structure with all data required to synchronize with k8s
type K8s struct {
	API                        *kubernetes.Clientset
	Logger                     utils.Logger
	DisableServiceExternalName bool // CVE-2021-25740
	RestConfig                 *rest.Config
}

// GetKubernetesClient returns new client that communicates with k8s
func GetKubernetesClient(disableServiceExternalName bool) (*K8s, error) {
	k8sLogger := utils.GetK8sAPILogger()
	if !TRACE_API {
		k8sLogger.SetLevel(utils.Info)
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	logger.Trace(config)
	if err != nil {
		logger.Panic(err)
	}
	return &K8s{
		API:                        clientset,
		Logger:                     k8sLogger,
		DisableServiceExternalName: disableServiceExternalName,
		RestConfig:                 config,
	}, nil
}

func getKubeConfig(kubeconfig string) *rest.Config {
	k8sLogger := utils.GetK8sAPILogger()
	if !TRACE_API {
		k8sLogger.SetLevel(utils.Info)
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		logger.Panic(err)
	}
	return config
}

// GetRemoteKubernetesClient returns new client that communicates with k8s
func GetRemoteKubernetesClient(kubeconfig string, disableServiceExternalName bool) (*K8s, error) {
	// create the clientset
	restConfig := getKubeConfig(kubeconfig)
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		logger.Panic(err)
	}
	return &K8s{
		API:                        clientset,
		Logger:                     logger,
		DisableServiceExternalName: disableServiceExternalName,
		RestConfig:                 restConfig,
	}, nil
}

func (k *K8s) EventsNamespaces(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.Namespace)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", NAMESPACE, obj)
					return
				}
				status := ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					// detect services that are in terminating state
					status = DELETED
				}
				item := &store.Namespace{
					Name:      data.GetName(),
					Endpoints: make(map[string]*store.Endpoints),
					Services:  make(map[string]*store.Service),
					Ingresses: make(map[string]*store.Ingress),
					Secret:    make(map[string]*store.Secret),
					Status:    status,
				}
				k.Logger.Tracef("%s %s: %s", NAMESPACE, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: NAMESPACE, Namespace: item.Name, Data: item}
			},
			DeleteFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.Namespace)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", NAMESPACE, obj)
					return
				}
				status := DELETED
				item := &store.Namespace{
					Name:      data.GetName(),
					Endpoints: make(map[string]*store.Endpoints),
					Services:  make(map[string]*store.Service),
					Ingresses: make(map[string]*store.Ingress),
					Secret:    make(map[string]*store.Secret),
					Status:    status,
				}
				k.Logger.Tracef("%s %s: %s", NAMESPACE, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: NAMESPACE, Namespace: item.Name, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1, ok := oldObj.(*corev1.Namespace)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", NAMESPACE, oldObj)
					return
				}
				data2, ok := newObj.(*corev1.Namespace)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", NAMESPACE, newObj)
					return
				}
				status := MODIFIED
				item1 := &store.Namespace{
					Name:   data1.GetName(),
					Status: status,
				}
				item2 := &store.Namespace{
					Name:   data2.GetName(),
					Status: status,
				}
				if item1.Name == item2.Name {
					return
				}
				k.Logger.Tracef("%s %s: %s", SERVICE, item2.Status, item2.Name)
				channel <- SyncDataEvent{SyncType: NAMESPACE, Namespace: item2.Name, Data: item2}
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventsEndpoints(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			item, err := k.convertToEndpoints(obj, ADDED)
			if errors.Is(err, ErrIgnored) {
				return
			}
			k.Logger.Tracef("%s %s: %s", ENDPOINTS, item.Status, item.Service)
			channel <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item.Namespace, Data: item}
		},
		DeleteFunc: func(obj interface{}) {
			item, err := k.convertToEndpoints(obj, DELETED)
			if errors.Is(err, ErrIgnored) {
				return
			}
			k.Logger.Tracef("%s %s: %s", ENDPOINTS, item.Status, item.Service)
			channel <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item.Namespace, Data: item}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			item1, err := k.convertToEndpoints(oldObj, EMPTY)
			if errors.Is(err, ErrIgnored) {
				return
			}
			item2, _ := k.convertToEndpoints(newObj, MODIFIED)
			if item2.Equal(item1) {
				return
			}
			// fix modified state for ones that are deleted,new,same
			k.Logger.Tracef("%s %s: %s", ENDPOINTS, item2.Status, item2.Service)
			channel <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item2.Namespace, Data: item2}
		},
	})
	go informer.Run(stop)
}

func (k *K8s) convertToEndpoints(obj interface{}, status store.Status) (*store.Endpoints, error) {
	data, ok := obj.(*corev1.Endpoints)
	if !ok {
		k.Logger.Errorf("%s: Invalid data from k8s api, %s", ENDPOINTS, obj)
		return nil, ErrIgnored
	}
	if data.GetNamespace() == "kube-system" {
		if data.ObjectMeta.Name == "kube-controller-manager" ||
			data.ObjectMeta.Name == "kube-scheduler" ||
			data.ObjectMeta.Name == "kubernetes-dashboard" ||
			data.ObjectMeta.Name == "kube-dns" {
			return nil, ErrIgnored
		}
	}
	if data.ObjectMeta.GetDeletionTimestamp() != nil {
		// detect endpoints that are in terminating state
		status = DELETED
	}
	item := &store.Endpoints{
		Namespace: data.GetNamespace(),
		Service:   data.GetName(),
		Ports:     make(map[string]*store.PortEndpoints),
		Status:    status,
	}
	for _, subset := range data.Subsets {
		for _, port := range subset.Ports {
			addresses := make(map[string]struct{})
			for _, address := range subset.Addresses {
				addresses[address.IP] = struct{}{}
			}
			item.Ports[port.Name] = &store.PortEndpoints{
				Port:        int64(port.Port),
				AddrCount:   len(addresses),
				AddrNew:     addresses,
				HAProxySrvs: make([]*store.HAProxySrv, 0, len(addresses)),
			}
		}
	}
	return item, nil
}

func (k *K8s) EventsIngressClass(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				item, err := store.ConvertToIngressClass(obj)
				if err != nil {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS_CLASS, obj)
					return
				}
				k.Logger.Tracef("%s %s: %s", INGRESS_CLASS, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: INGRESS_CLASS, Data: item}
			},
			DeleteFunc: func(obj interface{}) {
				item, err := store.ConvertToIngressClass(obj)
				if err != nil {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS_CLASS, obj)
					return
				}
				item.Status = DELETED
				k.Logger.Tracef("%s %s: %s", INGRESS_CLASS, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: INGRESS_CLASS, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				item1, err := store.ConvertToIngressClass(oldObj)
				if err != nil {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS_CLASS, oldObj)
					return
				}
				item1.Status = MODIFIED

				item2, err := store.ConvertToIngressClass(newObj)
				if err != nil {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, oldObj)
					return
				}
				item1.Status = MODIFIED

				if item2.Equal(item1) {
					return
				}
				k.Logger.Tracef("%s %s: %s", INGRESS_CLASS, item2.Status, item2.Name)
				channel <- SyncDataEvent{SyncType: INGRESS_CLASS, Data: item2}
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventsIngresses(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				item, err := store.ConvertToIngress(obj)
				if err != nil {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, obj)
					return
				}
				k.Logger.Tracef("%s %s: %s", INGRESS, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: INGRESS, Namespace: item.Namespace, Data: item}
			},
			DeleteFunc: func(obj interface{}) {
				item, err := store.ConvertToIngress(obj)
				if err != nil {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, obj)
					return
				}
				item.Status = DELETED
				k.Logger.Tracef("%s %s: %s", INGRESS, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: INGRESS, Namespace: item.Namespace, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				item1, err := store.ConvertToIngress(oldObj)
				if err != nil {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, oldObj)
					return
				}
				item1.Status = MODIFIED

				item2, err := store.ConvertToIngress(newObj)
				if err != nil {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, oldObj)
					return
				}
				item1.Status = MODIFIED

				if item2.Equal(item1) {
					return
				}
				k.Logger.Tracef("%s %s: %s", INGRESS, item2.Status, item2.Name)
				channel <- SyncDataEvent{SyncType: INGRESS, Namespace: item2.Namespace, Data: item2}
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventsServices(channel chan SyncDataEvent, ingChan chan ingstatus.SyncIngress, stop chan struct{}, informer cache.SharedIndexInformer, publishSvc *utils.NamespaceValue) {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			data, ok := obj.(*corev1.Service)
			if !ok {
				k.Logger.Errorf("%s: Invalid data from k8s api, %s", SERVICE, obj)
				return
			}
			if data.Spec.Type == corev1.ServiceTypeExternalName && k.DisableServiceExternalName {
				k.Logger.Tracef("forwarding to ExternalName Services for %v is disabled", data)
				return
			}
			status := ADDED
			if data.ObjectMeta.GetDeletionTimestamp() != nil {
				// detect services that are in terminating state
				status = DELETED
			}
			item := &store.Service{
				Namespace:   data.GetNamespace(),
				Name:        data.GetName(),
				Annotations: store.CopyAnnotations(data.ObjectMeta.Annotations),
				Ports:       []store.ServicePort{},
				Status:      status,
			}
			if data.Spec.Type == corev1.ServiceTypeExternalName {
				item.DNS = data.Spec.ExternalName
			}
			for _, sp := range data.Spec.Ports {
				item.Ports = append(item.Ports, store.ServicePort{
					Name:     sp.Name,
					Protocol: string(sp.Protocol),
					Port:     int64(sp.Port),
				})
			}
			k.Logger.Tracef("%s %s: %s", SERVICE, item.Status, item.Name)
			channel <- SyncDataEvent{SyncType: SERVICE, Namespace: item.Namespace, Data: item}
			if publishSvc != nil && publishSvc.Namespace == data.Namespace && publishSvc.Name == data.Name {
				ingChan <- ingstatus.SyncIngress{Service: data}
			}
		},
		DeleteFunc: func(obj interface{}) {
			data, ok := obj.(*corev1.Service)
			if !ok {
				k.Logger.Errorf("%s: Invalid data from k8s api, %s", SERVICE, obj)
				return
			}
			if data.Spec.Type == corev1.ServiceTypeExternalName && k.DisableServiceExternalName {
				return
			}
			status := DELETED
			item := &store.Service{
				Namespace:   data.GetNamespace(),
				Name:        data.GetName(),
				Annotations: store.CopyAnnotations(data.ObjectMeta.Annotations),
				Status:      status,
			}
			if data.Spec.Type == corev1.ServiceTypeExternalName {
				item.DNS = data.Spec.ExternalName
			}
			k.Logger.Tracef("%s %s: %s", SERVICE, item.Status, item.Name)
			channel <- SyncDataEvent{SyncType: SERVICE, Namespace: item.Namespace, Data: item}
			if publishSvc != nil && publishSvc.Namespace == data.Namespace && publishSvc.Name == data.Name {
				ingChan <- ingstatus.SyncIngress{Service: data}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			data1, ok := oldObj.(*corev1.Service)
			if !ok {
				k.Logger.Errorf("%s: Invalid data from k8s api, %s", SERVICE, oldObj)
				return
			}
			if data1.Spec.Type == corev1.ServiceTypeExternalName && k.DisableServiceExternalName {
				k.Logger.Tracef("forwarding to ExternalName Services for %v is disabled", data1)
				return
			}
			data2, ok := newObj.(*corev1.Service)
			if !ok {
				k.Logger.Errorf("%s: Invalid data from k8s api, %s", SERVICE, newObj)
				return
			}
			if data2.Spec.Type == corev1.ServiceTypeExternalName && k.DisableServiceExternalName {
				k.Logger.Tracef("forwarding to ExternalName Services for %v is disabled", data2)
				return
			}
			if publishSvc != nil && publishSvc.Namespace == data2.Namespace && publishSvc.Name == data2.Name {
				ingChan <- ingstatus.SyncIngress{Service: data2}
			}
			status := MODIFIED
			item1 := &store.Service{
				Namespace:   data1.GetNamespace(),
				Name:        data1.GetName(),
				Annotations: store.CopyAnnotations(data1.ObjectMeta.Annotations),
				Ports:       []store.ServicePort{},
				Status:      status,
			}
			if data1.Spec.Type == corev1.ServiceTypeExternalName {
				item1.DNS = data1.Spec.ExternalName
			}
			for _, sp := range data1.Spec.Ports {
				item1.Ports = append(item1.Ports, store.ServicePort{
					Name:     sp.Name,
					Protocol: string(sp.Protocol),
					Port:     int64(sp.Port),
				})
			}

			item2 := &store.Service{
				Namespace:   data2.GetNamespace(),
				Name:        data2.GetName(),
				Annotations: store.CopyAnnotations(data2.ObjectMeta.Annotations),
				Ports:       []store.ServicePort{},
				Status:      status,
			}
			if data2.Spec.Type == corev1.ServiceTypeExternalName {
				item2.DNS = data2.Spec.ExternalName
			}
			for _, sp := range data2.Spec.Ports {
				item2.Ports = append(item2.Ports, store.ServicePort{
					Name:     sp.Name,
					Protocol: string(sp.Protocol),
					Port:     int64(sp.Port),
				})
			}
			if item2.Equal(item1) {
				return
			}
			k.Logger.Tracef("%s %s: %s", SERVICE, item2.Status, item2.Name)
			channel <- SyncDataEvent{SyncType: SERVICE, Namespace: item2.Namespace, Data: item2}
		},
	})
	go informer.Run(stop)
}

func (k *K8s) EventsConfigfMaps(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.ConfigMap)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", CONFIGMAP, obj)
					return
				}
				status := ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					// detect services that are in terminating state
					status = DELETED
				}
				item := &store.ConfigMap{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: store.CopyAnnotations(data.Data),
					Status:      status,
				}
				k.Logger.Tracef("%s %s: %s", CONFIGMAP, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: item.Namespace, Data: item}
			},
			DeleteFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.ConfigMap)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", CONFIGMAP, obj)
					return
				}
				status := DELETED
				item := &store.ConfigMap{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: store.CopyAnnotations(data.Data),
					Status:      status,
				}
				k.Logger.Tracef("%s %s: %s", CONFIGMAP, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: item.Namespace, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1, ok := oldObj.(*corev1.ConfigMap)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", CONFIGMAP, oldObj)
					return
				}
				data2, ok := newObj.(*corev1.ConfigMap)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", CONFIGMAP, newObj)
					return
				}
				status := MODIFIED
				item1 := &store.ConfigMap{
					Namespace:   data1.GetNamespace(),
					Name:        data1.GetName(),
					Annotations: store.CopyAnnotations(data1.Data),
					Status:      status,
				}
				item2 := &store.ConfigMap{
					Namespace:   data2.GetNamespace(),
					Name:        data2.GetName(),
					Annotations: store.CopyAnnotations(data2.Data),
					Status:      status,
				}
				if item2.Equal(item1) {
					return
				}
				k.Logger.Tracef("%s %s: %s", CONFIGMAP, item2.Status, item2.Name)
				channel <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: item2.Namespace, Data: item2}
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventsSecrets(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.Secret)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", SECRET, obj)
					return
				}
				status := ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					// detect services that are in terminating state
					status = DELETED
				}
				item := &store.Secret{
					Namespace: data.GetNamespace(),
					Name:      data.GetName(),
					Data:      data.Data,
					Status:    status,
				}
				k.Logger.Tracef("%s %s: %s", SECRET, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: SECRET, Namespace: item.Namespace, Data: item}
			},
			DeleteFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.Secret)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", SECRET, obj)
					return
				}
				status := DELETED
				item := &store.Secret{
					Namespace: data.GetNamespace(),
					Name:      data.GetName(),
					Data:      data.Data,
					Status:    status,
				}
				k.Logger.Tracef("%s %s: %s", SECRET, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: SECRET, Namespace: item.Namespace, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1, ok := oldObj.(*corev1.Secret)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", SECRET, oldObj)
					return
				}
				data2, ok := newObj.(*corev1.Secret)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", SECRET, newObj)
					return
				}
				status := MODIFIED
				item1 := &store.Secret{
					Namespace: data1.GetNamespace(),
					Name:      data1.GetName(),
					Data:      data1.Data,
					Status:    status,
				}
				item2 := &store.Secret{
					Namespace: data2.GetNamespace(),
					Name:      data2.GetName(),
					Data:      data2.Data,
					Status:    status,
				}
				if item2.Equal(item1) {
					return
				}
				k.Logger.Tracef("%s %s: %s", SECRET, item2.Status, item2.Name)
				channel <- SyncDataEvent{SyncType: SECRET, Namespace: item2.Namespace, Data: item2}
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventPods(namespace, podPrefix string, resyncPeriod time.Duration, eventChan chan SyncDataEvent) {
	watchlist := cache.NewListWatchFromClient(k.API.CoreV1().RESTClient(), "pods", namespace, fields.Nothing())
	_, eController := cache.NewInformer(
		watchlist,
		&corev1.Pod{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				meta := obj.(*corev1.Pod).ObjectMeta
				if !strings.HasPrefix(meta.Name, podPrefix) {
					return
				}
				eventChan <- SyncDataEvent{SyncType: POD, Namespace: meta.Namespace, Data: store.PodEvent{Created: true}}
			},
			DeleteFunc: func(obj interface{}) {
				meta := obj.(*corev1.Pod).ObjectMeta

				if !strings.HasPrefix(meta.Name, podPrefix) {
					return
				}
				eventChan <- SyncDataEvent{SyncType: POD, Namespace: meta.Namespace, Data: store.PodEvent{}}
			},
		},
	)
	go eController.Run(wait.NeverStop)
}

func (k *K8s) IsNetworkingV1Beta1ApiSupported() bool {
	vi, _ := k.API.Discovery().ServerVersion()
	major, _ := utils.ParseInt(vi.Major)
	minor, _ := utils.ParseInt(vi.Minor)

	return major == 1 && minor >= 14 && minor < 22
}

func (k *K8s) IsNetworkingV1ApiSupported() bool {
	vi, _ := k.API.Discovery().ServerVersion()
	major, _ := utils.ParseInt(vi.Major)
	minor, _ := utils.ParseInt(vi.Minor)

	return major == 1 && minor >= 19
}
